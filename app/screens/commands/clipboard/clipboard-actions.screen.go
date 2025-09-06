package clipboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenClipboardActions handles navigation for the command actions.
func UpdateScreenClipboardActions(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	actions := []string{"Run", "Toggle Favorite", "Rename", "Save to Project", "Delete", "Back"}
	numOptions := len(actions)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.ClipboardActionIndex = (m.ClipboardActionIndex + numOptions - 1) % numOptions

	case "down", "j":
		m.ClipboardActionIndex = (m.ClipboardActionIndex + 1) % numOptions

	case "enter":
		selectedAction := actions[m.ClipboardActionIndex]
		cmdName := m.SelectedClipboardCommand

		switch selectedAction {
		case "Run":
			// Check if the command requires variables
			keys, err := commands.GetCommandVariableKeys(cmdName, m.ProjectPath, registry)
			if err != nil {
				// Handle error checking keys (e.g., template parsing failed)
				m.HistorySaveStatus = fmt.Sprintf("Error preparing command '%s': %v", cmdName, err)
				return m, nil
			}

			if len(keys) > 0 {
				// Command requires variables, go to prompt screen
				m.PendingCommand = cmdName
				m.MultipleVariables = len(keys) > 1 // Check if more than one key
				m.VariableKeys = keys
				m.CurrentVariableIndex = 0
				m.Variables = make(map[string]string)
				m.TempFilename = ""
				m.CurrentScreen = app.ScreenFilenamePrompt
				m.PromptOptionFocused = false // Ensure input is focused
				return m, cursor.Blink        // Start cursor blinking for input
			} else {
				// No variables needed, run directly
				m.HistorySaveStatus = fmt.Sprintf("Attempting to run: %s", cmdName)
				placeholders := make(map[string]string) // Empty placeholders
				runCmd := commands.RunCommand(cmdName, m.ProjectPath, placeholders, registry)
				return m, runCmd
			}
		case "Toggle Favorite":
			if registry != nil && registry.ClipboardCommands != nil {
				if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok {
					cmdSpec.IsFavorite = !cmdSpec.IsFavorite
					registry.ClipboardCommands[cmdName] = cmdSpec
					if err := registry.Save(); err != nil {
						fmt.Printf("Warning: Failed to save registry after toggling favorite: %v\n", err)
					}
					m.CurrentScreen = app.ScreenClipboardList
					m.ClipboardActionIndex = 0
					return m, nil
				}
			}
		case "Rename":
			m.CurrentScreen = app.ScreenRenameClipboard
			m.ClipboardRenameInput.SetValue(cmdName)
			m.ClipboardRenameInput.Focus()
			return m, textinput.Blink
		case "Save to Project":
			if registry != nil && m.ProjectPath != "" {
				if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok {
					localCmdDir := filepath.Join(m.ProjectPath, ".nextgen", "local-commands")
					if err := os.MkdirAll(localCmdDir, 0755); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Error creating dir: %v", err)
					} else {
						fileName := commands.ToKebabCase(cmdName) + ".json"
						targetPath := filepath.Join(localCmdDir, fileName)
						if err := os.WriteFile(targetPath, []byte(cmdSpec.Template), 0644); err != nil {
							m.HistorySaveStatus = fmt.Sprintf("Error saving file: %v", err)
						} else {
							m.HistorySaveStatus = fmt.Sprintf("Saved to project: %s", fileName)
							delete(registry.ClipboardCommands, cmdName)
							if saveErr := registry.Save(); saveErr != nil {
								fmt.Printf("Warning: Failed to save registry after saving/deleting clipboard command: %v\n", saveErr)
							}
						}
					}
				} else {
					m.HistorySaveStatus = "Error: Command not found in registry."
				}
			} else {
				m.HistorySaveStatus = "Error: Registry or Project Path unavailable."
			}
			m.CurrentScreen = app.ScreenClipboardList
			m.ClipboardActionIndex = 0
			return m, nil
		case "Delete":
			if registry != nil && registry.ClipboardCommands != nil {
				delete(registry.ClipboardCommands, cmdName)
				if err := registry.Save(); err != nil {
					fmt.Printf("Warning: Failed to save registry after deleting command: %v\n", err)
				}
				m.CurrentScreen = app.ScreenClipboardList
				m.SelectedClipboardCommand = ""
				m.ClipboardListIndex = 0
				m.ClipboardActionIndex = 0
				return m, nil
			}
		case "Back":
			m.CurrentScreen = app.ScreenClipboardList
			m.ClipboardActionIndex = 0
			return m, nil
		}

	case "esc", "b":
		m.CurrentScreen = app.ScreenClipboardList
		m.ClipboardActionIndex = 0
		return m, nil
	}

	return m, nil
}

// ViewScreenClipboardActions renders the actions available for a selected clipboard command.
func ViewScreenClipboardActions(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render(fmt.Sprintf("Actions for: %s", m.SelectedClipboardCommand)) + "\n"

	if registry == nil || registry.ClipboardCommands == nil {
		content := app.ChoiceStyle.Render("Registry not available. Press Esc/b to go back.")
		return lipgloss.JoinVertical(lipgloss.Left, header, content)
	}

	cmdSpec, exists := registry.ClipboardCommands[m.SelectedClipboardCommand]
	if !exists {
		content := app.ChoiceStyle.Render("Selected command no longer exists. Press Esc/b to go back.")
		return lipgloss.JoinVertical(lipgloss.Left, header, content)
	}

	favText := "Mark Favorite"
	if cmdSpec.IsFavorite {
		favText = "Unmark Favorite"
	}
	actions := []string{"Run", favText, "Rename", "Save to Project", "Delete", "Back"}

	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Action:") + "\n\n")

	for i, action := range actions {
		if i == m.ClipboardActionIndex {
			listBuilder.WriteString(app.HighlightStyle.Render("> "+action) + "\n")
		} else {
			listBuilder.WriteString(app.ChoiceStyle.Render("  "+action) + "\n")
		}
	}

	listPanel := lipgloss.NewStyle().Padding(1, 2).Render(listBuilder.String())

	footer := sharedScreens.Footer("↑↓ ←→ navigate", "enter to confirm", "ctrl+c quit")

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, listPanel, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
