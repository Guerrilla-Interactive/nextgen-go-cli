package clipboard

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

// GetSortedClipboardCommandNames retrieves sorted clipboard command names from the registry.
// Renamed to be exported.
func GetSortedClipboardCommandNames(registry *project.ProjectRegistry) []string {
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
	clipboardCmdNames := GetSortedClipboardCommandNames(registry)
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
	clipboardCmdNames := GetSortedClipboardCommandNames(registry)
	totalCmds := len(clipboardCmdNames)

	// --- Paginator Setup ---
	m.ClipboardPaginator.SetTotalPages(totalCmds)
	p := &m.ClipboardPaginator

	// --- Calculate index and page options ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back
	if m.ClipboardListIndex >= numOptionsOnPage {
		m.ClipboardListIndex = numOptionsOnPage - 1
	}
	if m.ClipboardListIndex < 0 { // Ensure index is not negative
		m.ClipboardListIndex = 0
	}
	var realIndex int // Index in the full list
	if totalCmds > 0 {
		realIndex = start + m.ClipboardListIndex
	} else {
		realIndex = -1 // No items
	}
	isBackSelected := m.ClipboardListIndex == numItemsOnPage

	// Update paginator first
	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left", "h":
		if totalCmds > 0 { // Only paginate if list not empty
			if p.Page != m.ClipboardPaginator.Page { // Check if page actually changed
				m.ClipboardListIndex = 0 // Reset index to top of new page
				m = updateClipboardListPreview(m, registry)
			}
		}
		return m, paginatorCmd

	case "right", "l":
		if totalCmds > 0 { // Only paginate if list not empty
			if p.Page != m.ClipboardPaginator.Page {
				m.ClipboardListIndex = 0 // Reset index to top of new page
				m = updateClipboardListPreview(m, registry)
			}
		}
		return m, paginatorCmd

	case "up", "k":
		if numOptionsOnPage > 0 { // Avoid modulo by zero
			newIndex := (m.ClipboardListIndex + numOptionsOnPage - 1) % numOptionsOnPage
			if newIndex != m.ClipboardListIndex {
				m.ClipboardListIndex = newIndex
				m = updateClipboardListPreview(m, registry) // <-- Update preview
			}
		}

	case "down", "j":
		if numOptionsOnPage > 0 { // Avoid modulo by zero
			newIndex := (m.ClipboardListIndex + 1) % numOptionsOnPage
			if newIndex != m.ClipboardListIndex {
				m.ClipboardListIndex = newIndex
				m = updateClipboardListPreview(m, registry) // <-- Update preview
			}
		}

	case "enter":
		if isBackSelected { // Back selected
			m.CurrentScreen = app.ScreenCommandsCategory
			m.ClipboardListIndex = 0 // Reset index
			return m, nil
		} else if realIndex >= 0 && realIndex < totalCmds { // Check against total commands
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

	clipboardCmdNames := GetSortedClipboardCommandNames(registry)
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

	// --- Calculate Paginator View Early ---
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}

	// --- Render List Items ---
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
	// listBuilder now only contains the command items

	// --- Render Back Button Separately ---
	backButtonView := ""
	if isBackSelected {
		backButtonView = app.HighlightStyle.Render("> Back")
	} else {
		backButtonView = app.ChoiceStyle.Render("  Back")
	}

	// --- Combine Left Pane Content ---
	// Join list, optional paginator, and back button
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
		Render(leftContentCombined) // Render the combined content

	// --- Right Pane: File Tree Preview ---
	previewContent := m.ClipboardListPreview
	if strings.TrimSpace(previewContent) == "" {
		// Minimal help panel when nothing selected
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
		title := app.SubtitleStyle.Render("Clipboard Preview")
		msg := muted.Render("Select a saved clipboard command on the left.")
		body := lipgloss.JoinVertical(lipgloss.Left, title, "", msg)
		previewContent = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Render(body)
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

	// Final join no longer includes paginatorView directly
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
