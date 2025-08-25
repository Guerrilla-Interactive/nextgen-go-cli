package native

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Helper function executeNativeCommand is no longer needed for this screen's purpose
// func executeNativeCommand(...) { ... }

// UpdateScreenNativeActions handles navigation for the native command actions.
func UpdateScreenNativeActions(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	actions := []string{"Run", "Back"} // Actions for built-in commands
	numOptions := len(actions)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.NativeActionIndex = (m.NativeActionIndex + numOptions - 1) % numOptions

	case "down", "j":
		m.NativeActionIndex = (m.NativeActionIndex + 1) % numOptions

	case "enter":
		if m.NativeActionIndex < 0 || m.NativeActionIndex >= numOptions {
			return m, nil
		}
		selectedAction := actions[m.NativeActionIndex]
		cmdName := m.SelectedNativeCommand // This is a built-in command name

		switch selectedAction {
		case "Run":
			// Check if the command requires variables
			keys, err := commands.GetCommandVariableKeys(cmdName, m.ProjectPath, registry)
			if err != nil {
				m.HistorySaveStatus = fmt.Sprintf("Error preparing command '%s': %v", cmdName, err)
				return m, nil
			}

			if len(keys) > 0 {
				// Command requires variables, go to prompt screen
				m.PendingCommand = cmdName
				m.MultipleVariables = len(keys) > 1
				m.VariableKeys = keys
				m.CurrentVariableIndex = 0
				m.Variables = make(map[string]string)
				m.TempFilename = ""
				m.CurrentScreen = app.ScreenFilenamePrompt
				m.PromptOptionFocused = false // Ensure input is focused
				return m, cursor.Blink        // Start cursor blinking
			} else {
				// No variables needed, run directly
				m.HistorySaveStatus = fmt.Sprintf("Attempting to run: %s", cmdName)
				placeholders := make(map[string]string)
				runCmd := commands.RunCommand(cmdName, m.ProjectPath, placeholders, registry)
				return m, runCmd
			}

		case "Back":
			m.CurrentScreen = app.ScreenNativeList
			m.NativeActionIndex = 0
			return m, nil
		}

	case "esc", "b": // Go back to List
		m.CurrentScreen = app.ScreenNativeList
		m.NativeActionIndex = 0
		return m, nil
	}

	return m, nil
}

// ViewScreenNativeActions renders the actions available for a selected native command.
func ViewScreenNativeActions(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render(fmt.Sprintf("Actions for: %s", m.SelectedNativeCommand)) + "\n"

	// Removed favorite logic
	actions := []string{"Run", "Back"}

	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Action:") + "\n\n")

	for i, action := range actions {
		if i == m.NativeActionIndex {
			listBuilder.WriteString(app.HighlightStyle.Render("> "+action) + "\n")
		} else {
			listBuilder.WriteString(app.ChoiceStyle.Render("  "+action) + "\n")
		}
	}

	listPanel := lipgloss.NewStyle().Padding(1, 2).Render(listBuilder.String())

	// Show status message if any
	status := ""
	if m.HistorySaveStatus != "" {
		status = app.ChoiceStyle.Render(m.HistorySaveStatus)
	}

	footer := app.HelpStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc/b to go back.")

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, listPanel, status, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
