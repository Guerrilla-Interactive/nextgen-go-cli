package native

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Helper to get sorted native command names FROM BUILT-IN LIST
func getSortedNativeCommandNames() []string {
	// Use the original logic based on commands.Commands
	names := make([]string, 0, len(commands.Commands))
	for _, cmdSpec := range commands.Commands {
		// Optionally filter out commands without templates?
		// For now, list all defined commands
		names = append(names, cmdSpec.Name)
	}
	sort.Strings(names)
	return names
}

// updateNativeListPreview generates the file tree preview for the selected native command.
func updateNativeListPreview(m app.Model) app.Model {
	m.NativeListPreview = "Loading preview..."
	nativeCmdNames := getSortedNativeCommandNames()
	totalCmds := len(nativeCmdNames)
	p := m.NativePaginator // Use the correct paginator
	start, _ := p.GetSliceBounds(totalCmds)
	numItemsOnPage := p.ItemsOnPage(totalCmds)
	isBackSelected := totalCmds == 0 || m.NativeListIndex == numItemsOnPage // Index relative to page items + Back
	var realIndex int
	if totalCmds > 0 {
		realIndex = start + m.NativeListIndex
	} else {
		realIndex = -1
	}

	if isBackSelected || realIndex < 0 || realIndex >= totalCmds {
		m.NativeListPreview = "(Select a command)"
		return m
	}

	cmdName := nativeCmdNames[realIndex]
	// Use default placeholders for preview
	// Use the new function to get keys, needs registry... but preview doesn't have it easily.
	// Fallback to simple placeholder for now for preview.
	// TODO: Refactor preview generation or pass registry if needed for accurate preview placeholders.
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
	nativeCmdNames := getSortedNativeCommandNames() // Use built-in list
	totalCmds := len(nativeCmdNames)

	// --- Paginator Setup ---
	m.NativePaginator.SetTotalPages(totalCmds)
	p := &m.NativePaginator

	// --- Calculate index and page options ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back
	if m.NativeListIndex >= numOptionsOnPage {
		m.NativeListIndex = numOptionsOnPage - 1
	}
	if m.NativeListIndex < 0 {
		m.NativeListIndex = 0
	}
	var realIndex int // Index in the full list
	if totalCmds > 0 {
		realIndex = start + m.NativeListIndex
	} else {
		realIndex = -1 // No items
	}
	isBackSelected := totalCmds == 0 || m.NativeListIndex == numItemsOnPage

	// Update paginator first
	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left", "h":
		if totalCmds > 0 { // Only paginate if list not empty
			oldPage := p.Page
			*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyLeft})
			if p.Page != oldPage {
				m.NativeListIndex = 0
				m = updateNativeListPreview(m)
			}
		}
		return m, paginatorCmd

	case "right", "l":
		if totalCmds > 0 { // Only paginate if list not empty
			oldPage := p.Page
			*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyRight})
			if p.Page != oldPage {
				m.NativeListIndex = 0
				m = updateNativeListPreview(m)
			}
		}
		return m, paginatorCmd

	case "up", "k":
		if numOptionsOnPage > 0 { // Avoid modulo by zero if list is empty
			newIndex := (m.NativeListIndex + numOptionsOnPage - 1) % numOptionsOnPage
			if newIndex != m.NativeListIndex {
				m.NativeListIndex = newIndex
				m = updateNativeListPreview(m) // Update preview
			}
		}

	case "down", "j":
		if numOptionsOnPage > 0 { // Avoid modulo by zero if list is empty
			newIndex := (m.NativeListIndex + 1) % numOptionsOnPage
			if newIndex != m.NativeListIndex {
				m.NativeListIndex = newIndex
				m = updateNativeListPreview(m) // Update preview
			}
		}

	case "enter":
		if isBackSelected {
			m.CurrentScreen = app.ScreenCommandsCategory
			m.NativeListIndex = 0
			return m, nil
		} else if realIndex >= 0 && realIndex < totalCmds { // Check index is valid
			cmdName := nativeCmdNames[realIndex]
			m.SelectedNativeCommand = cmdName // Store selected command
			m.CurrentScreen = app.ScreenNativeActions
			m.NativeActionIndex = 0
			m.HistorySaveStatus = "" // Clear status before showing actions
			return m, nil
		}

	case "esc", "b":
		m.CurrentScreen = app.ScreenCommandsCategory
		m.NativeListIndex = 0
		return m, nil
	}

	return m, paginatorCmd
}

// ViewScreenNativeList renders the list of native commands with preview.
func ViewScreenNativeList(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Built-in Commands") + "\n" // Updated title

	nativeCmdNames := getSortedNativeCommandNames() // Use built-in list
	totalCmds := len(nativeCmdNames)

	// --- Get paginated items ---
	p := m.NativePaginator
	start, end := p.GetSliceBounds(totalCmds)
	paginatedCmds := []string{}
	if start < end {
		paginatedCmds = nativeCmdNames[start:end]
	}
	numItemsOnPage := len(paginatedCmds)
	isBackSelected := totalCmds == 0 || m.NativeListIndex == numItemsOnPage

	// --- Calculate Paginator View Early ---
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}

	// --- Render List Items ---
	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Command:") + "\n\n")
	if totalCmds == 0 {
		listBuilder.WriteString(app.ChoiceStyle.Render("  (No built-in commands found)") + "\n")
	} else {
		for i, name := range paginatedCmds {
			// No favorite status for built-in commands
			prefix := "  "

			if i == m.NativeListIndex {
				listBuilder.WriteString(app.HighlightStyle.Render("> "+prefix+name) + "\n")
			} else {
				listBuilder.WriteString(app.ChoiceStyle.Render("  "+prefix+name) + "\n")
			}
		}
	}
	// listBuilder now only contains command items

	// --- Render Back Button Separately ---
	backButtonView := ""
	if isBackSelected {
		backButtonView = app.HighlightStyle.Render("> Back")
	} else {
		backButtonView = app.ChoiceStyle.Render("  Back")
	}

	// --- Combine Left Pane Content ---
	leftContentItems := []string{listBuilder.String()}
	if paginatorView != "" {
		leftContentItems = append(leftContentItems, lipgloss.NewStyle().MarginTop(1).Render(paginatorView))
	}
	leftContentItems = append(leftContentItems, backButtonView)
	leftContentCombined := lipgloss.JoinVertical(lipgloss.Left, leftContentItems...)

	// --- Left Panel Styling & Rendering ---
	leftPanelWidth := 40
	leftPanel := lipgloss.NewStyle().
		Width(leftPanelWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Render(leftContentCombined)

	// --- Right Pane: File Tree Preview ---
	previewContent := m.NativeListPreview
	if previewContent == "" {
		previewContent = app.HelpStyle.Render("Select a command to see its file tree preview.")
	}

	// --- Truncate Preview Content ---
	const maxPreviewLines = 12
	lines := strings.Split(previewContent, "\n")
	if len(lines) > maxPreviewLines {
		previewContent = strings.Join(lines[:maxPreviewLines], "\n")
		previewContent += "\n... (truncated)"
	}

	// Prepend header
	folderName := filepath.Base(m.ProjectPath)
	headerPreview := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContent = headerPreview + "\n\n" + previewContent // Use the potentially truncated content

	// Define right panel style WITHOUT explicit width
	rightPanelStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Height(lipgloss.Height(leftPanel)). // Match height roughly
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
	rightPanel := rightPanelStyle.Render(previewContent)

	// --- Combine, Footer ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	footer := sharedScreens.Footer("↑↓ ←→ navigate", "enter to confirm", "ctrl+c quit")

	// Combine list, paginator, footer
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
