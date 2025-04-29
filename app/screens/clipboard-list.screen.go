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

// Helper to get sorted keys from the ClipboardCommands map
func getSortedClipboardCommandNames(registry *project.ProjectRegistry) []string {
	if registry == nil || registry.ClipboardCommands == nil {
		return []string{}
	}
	names := make([]string, 0, len(registry.ClipboardCommands))
	for name := range registry.ClipboardCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// updateClipboardListPreview generates the file tree preview for the selected clipboard command.
func updateClipboardListPreview(m app.Model, registry *project.ProjectRegistry) app.Model {
	m.ClipboardListPreview = "Loading preview..."
	clipboardCmdNames := getSortedClipboardCommandNames(registry)
	totalCmds := len(clipboardCmdNames)
	start, _ := m.ClipboardPaginator.GetSliceBounds(totalCmds)
	realIndex := start + m.ClipboardListIndex
	isBackSelected := m.ClipboardListIndex == m.ClipboardPaginator.ItemsOnPage(totalCmds)

	if isBackSelected || realIndex >= totalCmds {
		m.ClipboardListPreview = "(Select a command)"
		return m
	}

	cmdName := clipboardCmdNames[realIndex]
	cmdSpec, exists := registry.ClipboardCommands[cmdName]
	if !exists {
		m.ClipboardListPreview = "(Error: Command spec not found in registry)"
		return m
	}

	// Use default placeholders for preview
	placeholderMap := commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
	// Use GeneratePreviewFileTreeFromBytes with the stored template content
	pv, err := commands.GeneratePreviewFileTreeFromBytes([]byte(cmdSpec.Template), placeholderMap, m.ProjectPath)

	if err == nil && strings.TrimSpace(pv) != "" {
		m.ClipboardListPreview = pv
	} else {
		m.ClipboardListPreview = fmt.Sprintf("(Preview generation failed for %s)", cmdName)
		if err != nil {
			m.ClipboardListPreview += fmt.Sprintf("\nError: %v", err)
		}
	}
	return m
}

// UpdateScreenClipboardList handles navigation for the list of saved clipboard commands.
func UpdateScreenClipboardList(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	clipboardCmdNames := getSortedClipboardCommandNames(registry)
	totalCmds := len(clipboardCmdNames)

	// --- Paginator Setup ---
	m.ClipboardPaginator.SetTotalPages(totalCmds)
	p := &m.ClipboardPaginator

	// --- Calculate real index based on paginator and list index ---
	// The list index (m.ClipboardListIndex) now refers to the item on the *current page*.
	start, end := p.GetSliceBounds(totalCmds)
	// Clamp list index to the number of items on the current page + Back button
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back button
	if m.ClipboardListIndex >= numOptionsOnPage {
		m.ClipboardListIndex = numOptionsOnPage - 1
	}
	realIndex := start + m.ClipboardListIndex // Index in the full clipboardCmdNames list
	isBackSelected := m.ClipboardListIndex == numItemsOnPage

	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	var cmd tea.Cmd
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left", "h":
		*p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyLeft}) // Use paginator update
		return m, cmd

	case "right", "l":
		*p, cmd = p.Update(tea.KeyMsg{Type: tea.KeyRight}) // Use paginator update
		return m, cmd

	case "up", "k":
		newIndex := (m.ClipboardListIndex + numOptionsOnPage - 1) % numOptionsOnPage
		if newIndex != m.ClipboardListIndex {
			m.ClipboardListIndex = newIndex
			m = updateClipboardListPreview(m, registry) // <-- Update preview
		}

	case "down", "j":
		newIndex := (m.ClipboardListIndex + 1) % numOptionsOnPage
		if newIndex != m.ClipboardListIndex {
			m.ClipboardListIndex = newIndex
			m = updateClipboardListPreview(m, registry) // <-- Update preview
		}

	case "enter":
		if isBackSelected { // Back selected
			m.CurrentScreen = app.ScreenCommandsCategory
			m.ClipboardListIndex = 0 // Reset index
			return m, nil
		} else if realIndex < totalCmds { // Check against total commands
			// Command selected - Go to Actions screen
			m.SelectedClipboardCommand = clipboardCmdNames[realIndex]
			m.CurrentScreen = app.ScreenClipboardActions
			m.ClipboardActionIndex = 0 // Reset action index
			return m, nil
		}

	case "esc", "b": // Go back to Categories
		m.CurrentScreen = app.ScreenCommandsCategory
		m.ClipboardListIndex = 0 // Reset index
		return m, nil
	}

	return m, paginatorCmd
}

// ViewScreenClipboardList renders the list of saved clipboard commands with preview.
func ViewScreenClipboardList(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Saved Clipboard Commands") + "\n"

	clipboardCmdNames := getSortedClipboardCommandNames(registry)
	totalCmds := len(clipboardCmdNames)

	// --- Get paginated items ---
	p := m.ClipboardPaginator
	start, end := p.GetSliceBounds(totalCmds)
	paginatedCmds := []string{}
	if start < end {
		paginatedCmds = clipboardCmdNames[start:end]
	}
	numItemsOnPage := len(paginatedCmds)
	isBackSelected := m.ClipboardListIndex == numItemsOnPage

	// --- Render List ---
	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Command:") + "\n\n")

	if len(paginatedCmds) == 0 {
		listBuilder.WriteString(app.ChoiceStyle.Render("  (No clipboard commands saved yet)") + "\n")
	} else {
		for i, name := range paginatedCmds {
			// Check favorite status
			prefix := "  "
			if registry != nil {
				if cmdSpec, ok := registry.ClipboardCommands[name]; ok && cmdSpec.IsFavorite {
					prefix = "⭐ " // Add star for favorites
				}
			}

			if i == m.ClipboardListIndex {
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
	previewContent := m.ClipboardListPreview
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

	footer := app.HelpStyle.Render("Use ↑/↓/←/→ to navigate, Enter to select, Esc/b to go back.")

	// Combine list, paginator, footer
	return lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", paginatorView, "\n", footer)
}
