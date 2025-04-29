package screens

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Helper to get sorted native command names
func getSortedNativeCommandNames() []string {
	names := make([]string, 0, len(commands.Commands))
	for _, cmdSpec := range commands.Commands {
		// Optionally filter out commands without templates?
		// if cmdSpec.TemplatePath != "" {
		names = append(names, cmdSpec.Name)
		// }
	}
	sort.Strings(names)
	return names
}

// updateNativeListPreview generates the file tree preview for the selected native command.
func updateNativeListPreview(m app.Model) app.Model {
	m.NativeListPreview = "Loading preview..."
	nativeCmdNames := getSortedNativeCommandNames()
	totalCmds := len(nativeCmdNames)
	start, _ := m.NativePaginator.GetSliceBounds(totalCmds)
	realIndex := start + m.NativeListIndex
	isBackSelected := m.NativeListIndex == m.NativePaginator.ItemsOnPage(totalCmds)

	if isBackSelected || realIndex >= totalCmds {
		m.NativeListPreview = "(Select a command)"
		return m
	}

	cmdName := nativeCmdNames[realIndex]
	// Use default placeholders for preview
	placeholderMap := commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})

	pv, err := commands.GeneratePreviewFileTree(cmdName, placeholderMap, m.ProjectPath)
	if err == nil && strings.TrimSpace(pv) != "" {
		m.NativeListPreview = pv
	} else {
		m.NativeListPreview = fmt.Sprintf("(Preview generation failed for %s)", cmdName)
		if err != nil {
			m.NativeListPreview += fmt.Sprintf("\nError: %v", err)
		}
	}
	return m
}

// UpdateScreenNativeList handles navigation for the list of native commands.
func UpdateScreenNativeList(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	nativeCmdNames := getSortedNativeCommandNames()
	totalCmds := len(nativeCmdNames)

	// --- Paginator Setup ---
	m.NativePaginator.SetTotalPages(totalCmds)
	p := &m.NativePaginator

	// --- Calculate real index and page options ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back
	if m.NativeListIndex >= numOptionsOnPage {
		m.NativeListIndex = numOptionsOnPage - 1
	}
	realIndex := start + m.NativeListIndex
	isBackSelected := m.NativeListIndex == numItemsOnPage

	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left", "h":
		*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyLeft})
		return m, paginatorCmd

	case "right", "l":
		*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyRight})
		return m, paginatorCmd

	case "up", "k":
		newIndex := (m.NativeListIndex + numOptionsOnPage - 1) % numOptionsOnPage
		if newIndex != m.NativeListIndex {
			m.NativeListIndex = newIndex
			m = updateNativeListPreview(m) // Update preview
		}

	case "down", "j":
		newIndex := (m.NativeListIndex + 1) % numOptionsOnPage
		if newIndex != m.NativeListIndex {
			m.NativeListIndex = newIndex
			m = updateNativeListPreview(m) // Update preview
		}

	case "enter":
		if isBackSelected { // Back selected
			m.CurrentScreen = app.ScreenCommandsCategory
			m.NativeListIndex = 0 // Reset index
			return m, nil
		} else if realIndex < totalCmds { // Check against total commands
			// Command selected - Go to Native Actions screen
			cmdName := nativeCmdNames[realIndex]
			m.SelectedNativeCommand = cmdName // Store selected command
			m.CurrentScreen = app.ScreenNativeActions
			m.NativeActionIndex = 0 // Reset action index
			return m, nil
		}

	case "esc", "b": // Go back to Categories
		m.CurrentScreen = app.ScreenCommandsCategory
		m.NativeListIndex = 0 // Reset index
		return m, nil
	}

	return m, paginatorCmd
}

// ViewScreenNativeList renders the list of native commands with preview.
func ViewScreenNativeList(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Native Commands") + "\n"

	nativeCmdNames := getSortedNativeCommandNames()
	totalCmds := len(nativeCmdNames)

	// --- Get paginated items ---
	p := m.NativePaginator
	start, end := p.GetSliceBounds(totalCmds)
	paginatedCmds := []string{}
	if start < end {
		paginatedCmds = nativeCmdNames[start:end]
	}
	numItemsOnPage := len(paginatedCmds)
	isBackSelected := m.NativeListIndex == numItemsOnPage

	// --- Render List ---
	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Command:") + "\n\n")

	if len(paginatedCmds) == 0 {
		listBuilder.WriteString(app.ChoiceStyle.Render("  (No native commands found)") + "\n")
	} else {
		for i, name := range paginatedCmds {
			// Check favorite status
			prefix := "  "
			if registry != nil {
				if isFav, ok := registry.FavoriteNativeCommands[name]; ok && isFav {
					prefix = "⭐ " // Add star for favorites
				}
			}

			if i == m.NativeListIndex {
				listBuilder.WriteString(app.HighlightStyle.Render("> "+prefix+name) + "\n")
			} else {
				listBuilder.WriteString(app.ChoiceStyle.Render("  "+prefix+name) + "\n") // Indent non-selected favorites too
			}
		}
	}
	listBuilder.WriteString("\n") // Spacer

	// Add Back button
	if isBackSelected {
		listBuilder.WriteString(app.HighlightStyle.Render("> Back") + "\n")
	} else {
		listBuilder.WriteString(app.ChoiceStyle.Render("  Back") + "\n")
	}

	leftPanel := lipgloss.NewStyle().Padding(1, 2).Width(40).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Render(listBuilder.String())

	// --- Right Pane: File Tree Preview ---
	previewContent := m.NativeListPreview
	if previewContent == "" {
		previewContent = app.HelpStyle.Render("Select a command to see its file tree preview.")
	}

	// Prepend header
	folderName := filepath.Base(m.ProjectPath)
	headerPreview := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContent = headerPreview + "\n\n" + previewContent

	// Apply consistent padding, no border
	rightPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.TerminalWidth - 40 - 8).
		Height(lipgloss.Height(leftPanel)).
		Render(previewContent)

	// --- Combine ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	// --- Paginator View ---
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}

	footer := app.HelpStyle.Render("Use ↑/↓/←/→ to navigate, Enter to select (NYI), Esc/b to go back.")

	// Combine list, paginator, footer
	return lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", paginatorView, "\n", footer)
}
