package screens

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	clipboardScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/clipboard" // Import clipboard screen
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenCommandsCategory handles navigation for the command categories.
func UpdateScreenCommandsCategory(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	// Categories: Clipboard, Native (placeholder), Project Commands, Back
	numOptions := 4

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.CommandsCategoryIndex = (m.CommandsCategoryIndex + numOptions - 1) % numOptions

	case "down", "j":
		m.CommandsCategoryIndex = (m.CommandsCategoryIndex + 1) % numOptions

	case "enter":
		switch m.CommandsCategoryIndex {
		case 0: // Clipboard Commands
			m.CurrentScreen = app.ScreenClipboardList
			m.ClipboardListIndex = 0 // Reset index for the new screen
			return m, nil
		case 1: // Native Commands
			m.CurrentScreen = app.ScreenNativeList
			m.NativeListIndex = 0 // Reset index
			return m, nil
		case 2: // Project Commands
			m.CurrentScreen = app.ScreenProjectCommandsList
			// TODO: Reset ProjectCommandsListIndex here if needed
			return m, nil
		case 3: // Back
			m.CurrentScreen = app.ScreenSettings
			m.CommandsCategoryIndex = 0
			return m, nil
		}

	case "esc", "b": // Go back to Settings
		m.CurrentScreen = app.ScreenSettings
		m.CommandsCategoryIndex = 0
		return m, nil
	}

	return m, nil
}

// ViewScreenCommandsCategory renders the list of command categories with a preview pane.
func ViewScreenCommandsCategory(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Manage Commands") + "\n"

	categories := []string{"Clipboard Commands", "Native Commands", "Project Commands", "Back"}

	// --- Left Pane: Categories ---
	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Category:") + "\n\n")
	for i, cat := range categories {
		if i == m.CommandsCategoryIndex {
			listBuilder.WriteString(app.HighlightStyle.Render("> "+cat) + "\n")
		} else {
			listBuilder.WriteString(app.ChoiceStyle.Render("  "+cat) + "\n")
		}
	}
	leftPanel := lipgloss.NewStyle().
		Width(30). // Fixed width for left nav
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Render(listBuilder.String())

	// --- Right Pane: Preview ---
	var previewContent string
	switch m.CommandsCategoryIndex {
	case 0: // Clipboard Commands Preview
		var pb strings.Builder
		pb.WriteString(app.SubtitleStyle.Render("Saved Clipboard Commands") + "\n\n")
		clipboardCmdNames := clipboardScreen.GetSortedClipboardCommandNames(registry)
		if len(clipboardCmdNames) == 0 {
			pb.WriteString(app.ChoiceStyle.Render("  (No commands saved yet)"))
		} else {
			limit := 7
			for i, name := range clipboardCmdNames {
				if i >= limit {
					pb.WriteString(app.ChoiceStyle.Render("  ...") + "\n")
					break
				}
				pb.WriteString(fmt.Sprintf("- %s\n", name))
			}
		}
		previewContent = pb.String()
	case 1: // Native Commands Preview
		var pb strings.Builder
		pb.WriteString(app.SubtitleStyle.Render("Built-in Commands") + "\n\n")
		nativeCmdNames := commands.AllCommandNames()
		if len(nativeCmdNames) == 0 {
			pb.WriteString(app.ChoiceStyle.Render("  (No built-in commands found)"))
		} else {
			limit := 7
			for i, name := range nativeCmdNames {
				if i >= limit {
					pb.WriteString(app.ChoiceStyle.Render("  ...") + "\n")
					break
				}
				pb.WriteString(fmt.Sprintf("- %s\n", name))
			}
		}
		previewContent = pb.String()
	case 2: // Project Commands Preview
		previewContent = app.HelpStyle.Render("View commands saved locally in .nextgen/local-commands")
	case 3: // Back Preview
		previewContent = app.HelpStyle.Render("Return to the Project Stats screen.")
	default:
		previewContent = ""
	}

	rightPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.TerminalWidth - 30 - 8).    // Adjust width
		Height(lipgloss.Height(leftPanel)). // Match height roughly
		Border(lipgloss.RoundedBorder()).
		Render(previewContent)

	// --- Combine & Footer ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	footer := sharedScreens.Footer("↑↓ ←→ navigate", "enter to confirm", "ctrl+c quit")

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
