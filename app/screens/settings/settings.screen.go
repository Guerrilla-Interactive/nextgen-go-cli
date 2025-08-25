package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenSettings handles input on the Settings screen.
func UpdateScreenSettings(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	// Updated categories: Project Info, Command History, Manage Commands, Back
	numOptions := 4 // Reduced number of options

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		// Use SettingsScreenIndex
		m.SettingsScreenIndex = (m.SettingsScreenIndex + numOptions - 1) % numOptions

	case "down", "j":
		// Use SettingsScreenIndex
		m.SettingsScreenIndex = (m.SettingsScreenIndex + 1) % numOptions

	case "enter":
		switch m.SettingsScreenIndex {
		case 0: // Project Info (now covers old 0, 1, 2)
			// No action, just shows combined view
		case 1: // Command History (new index 1, old 3)
			m.CurrentScreen = app.ScreenCommandHistory
			m.HistoryScreenIndex = 0 // Reset index for the target screen
			return m, nil
		case 2: // Manage Commands (new index 2, old 4)
			m.CurrentScreen = app.ScreenCommandsCategory
			m.CommandsCategoryIndex = 0 // Reset index for the target screen
			return m, nil
		case 3: // Back (new index 3, old 5)
			m.CurrentScreen = app.ScreenMain
			m.SettingsScreenIndex = 0 // Reset index for this screen
			return m, nil
		}

	case "esc", "b": // Go back to Main
		m.CurrentScreen = app.ScreenMain
		m.SettingsScreenIndex = 0 // Reset index for this screen
		return m, nil
	}

	return m, nil
}

// ViewSettingsScreen renders the interactive settings screen.
func ViewSettingsScreen(m app.Model, registry *project.ProjectRegistry) string {
	// Rename title
	leftHeader := app.TitleStyle.Render("Settings")

	// --- Left Pane: Navigation ---\
	// Updated navigation items
	navItems := []string{"Project Info", "Command History", "Manage Commands", "Back"}
	var leftBuilder strings.Builder

	for i, item := range navItems {
		// Use SettingsScreenIndex
		if i == m.SettingsScreenIndex {
			leftBuilder.WriteString(app.HighlightStyle.Render("> "+item) + "\n")
		} else {
			leftBuilder.WriteString(app.ChoiceStyle.Render("  "+item) + "\n")
		}
	}
	leftPanelWidth := 50 // Define fixed width for left panel
	leftPanelContent := lipgloss.JoinVertical(lipgloss.Left, leftHeader, leftBuilder.String())
	leftPanel := lipgloss.NewStyle().
		Width(leftPanelWidth). // Apply fixed width
		Padding(2, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Render(leftPanelContent)

	// --- Right Pane: Details Preview ---\
	var previewContent string
	// Use SettingsScreenIndex with updated logic
	switch m.SettingsScreenIndex {
	case 0: // Project Info (now covers old 0, 1, 2)
		var combinedBuilder strings.Builder
		// Project Info (Path, Type)
		combinedBuilder.WriteString(app.SubtitleStyle.Render("Project Overview") + "\n\n")
		if m.ProjectPath != "" {
			combinedBuilder.WriteString("Path: " + app.PathStyle.Render(m.ProjectPath) + "\n")
			if registry != nil {
				if info, found := registry.GetProject(m.ProjectPath); found && info.Type != "" {
					combinedBuilder.WriteString(fmt.Sprintf("Type: %s\n", info.Type))
				}
			}
		} else {
			combinedBuilder.WriteString(app.ChoiceStyle.Render("Path not available.") + "\n")
		}
		combinedBuilder.WriteString("\n") // Separator
		// Detected Packages
		combinedBuilder.WriteString(app.SubtitleStyle.Render("Detected Packages") + "\n") // No extra newline needed here
		if len(m.RecognizedPkgs) > 0 {
			combinedBuilder.WriteString(app.SummarizeFullProjectStats(m.RecognizedPkgs))
		} else {
			combinedBuilder.WriteString(app.ChoiceStyle.Render("No packages detected.") + "\n")
		}
		combinedBuilder.WriteString("\n") // Separator
		// Project Usage (Count, Last Access)
		combinedBuilder.WriteString(app.SubtitleStyle.Render("Project Usage") + "\n\n")
		if registry != nil && m.ProjectPath != "" {
			if info, found := registry.GetProject(m.ProjectPath); found {
				combinedBuilder.WriteString(fmt.Sprintf("- Count: %d\n", info.UsageCount))
				lastAccess := time.Unix(info.LastAccessTime, 0)
				combinedBuilder.WriteString(fmt.Sprintf("- Last Access: %s\n", lastAccess.Format("Jan 2, 2006 at 3:04 PM")))
			} else {
				combinedBuilder.WriteString(app.ChoiceStyle.Render("  (Project usage not recorded yet)\n"))
			}
		} else {
			combinedBuilder.WriteString(app.ChoiceStyle.Render("  (Registry or Project Path not available)\n"))
		}
		previewContent = combinedBuilder.String()
	case 1: // Command History Preview (new index 1, old 3)
		previewContent = app.HelpStyle.Render("View the history of commands run in this project.")
	case 2: // Manage Commands Preview (new index 2, old 4)
		previewContent = app.HelpStyle.Render("Manage saved clipboard, native, and project-specific commands.")
	case 3: // Back Preview (new index 3, old 5)
		previewContent = app.HelpStyle.Render("Return to the main command screen.")
	default:
		previewContent = "" // Should not happen
	}

	// --- Compute available height and bottom-align panels ---
	footer := app.HelpStyle.Render("Use ↑/↓ to navigate categories, Enter to select, Esc/b to go back.")
	footerHeight := lipgloss.Height(footer)
	availableRowHeight := m.TerminalHeight - footerHeight - 1
	if availableRowHeight < 10 {
		availableRowHeight = 10
	}

	// Left column: header (top) + left box (bottom-aligned within remaining height)
	leftHeaderHeight := lipgloss.Height(leftHeader)
	leftBoxHeight := availableRowHeight - leftHeaderHeight
	if leftBoxHeight < 3 {
		leftBoxHeight = 3
	}
	leftRendered := leftPanel
	leftColumn := lipgloss.Place(leftPanelWidth, availableRowHeight, lipgloss.Left, lipgloss.Bottom, leftRendered)

	// Right panel bottom-aligned within the full row height
	rightInner := lipgloss.NewStyle().Padding(2, 2).Border(lipgloss.RoundedBorder()).Render(previewContent)
	rightPanel := lipgloss.Place(lipgloss.Width(rightInner), availableRowHeight, lipgloss.Left, lipgloss.Bottom, rightInner)

	// --- Combine & Footer ---\
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, "  ", rightPanel)
	finalView := lipgloss.JoinVertical(lipgloss.Left, combinedPanes, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
