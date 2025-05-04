package projectCmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"          // For ToKebabCase
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands" // Import commands package
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"  // Import shared
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenProjectCommandActions handles navigation for the project command actions.
func UpdateScreenProjectCommandActions(m app.Model, msg tea.Msg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	actions := []string{"Run", "Toggle Favorite", "Delete", "Back"}
	numOptions := len(actions)

	switch msg := msg.(type) { // Use type switch on the message
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			m.ProjectCommandActionIndex = (m.ProjectCommandActionIndex + numOptions - 1) % numOptions
			return m, nil // Return immediately after state change

		case "down", "j":
			m.ProjectCommandActionIndex = (m.ProjectCommandActionIndex + 1) % numOptions
			return m, nil // Return immediately after state change

		case "enter":
			// Check index bounds before accessing actions
			if m.ProjectCommandActionIndex < 0 || m.ProjectCommandActionIndex >= numOptions {
				return m, nil // Should not happen, but safe check
			}
			selectedAction := actions[m.ProjectCommandActionIndex]
			cmdName := m.SelectedProjectCommand // This is the base name (e.g., hello-world)

			switch selectedAction {
			case "Run":
				// Check if the command requires variables
				keys, err := commands.GetCommandVariableKeys(cmdName, m.ProjectPath, registry)
				if err != nil {
					// Handle error checking keys
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
			case "Toggle Favorite":
				if registry != nil {
					if registry.FavoriteProjectCommands == nil {
						registry.FavoriteProjectCommands = make(map[string]bool)
					}
					if _, isFav := registry.FavoriteProjectCommands[cmdName]; isFav {
						delete(registry.FavoriteProjectCommands, cmdName)
					} else {
						registry.FavoriteProjectCommands[cmdName] = true
					}
					if err := registry.Save(); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Warning: Failed to save registry: %v", err)
					}
					m.CurrentScreen = app.ScreenProjectCommandsList
					m.ProjectCommandActionIndex = 0
					return m, nil
				}
			case "Delete":
				if m.ProjectPath != "" {
					localCmdDir := filepath.Join(m.ProjectPath, ".nextgen", "local-commands")
					fileName := cmdName + ".json" // Assuming cmdName is already kebab-case here
					targetPath := filepath.Join(localCmdDir, fileName)
					if err := os.Remove(targetPath); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Error deleting file %s: %v", fileName, err)
					} else {
						m.HistorySaveStatus = fmt.Sprintf("Deleted project command: %s", fileName)
						// Also remove from favorites if it was favorited
						if registry != nil && registry.FavoriteProjectCommands != nil {
							delete(registry.FavoriteProjectCommands, cmdName)
							_ = registry.Save() // Attempt to save favorite change, ignore error for now
						}
					}
				} else {
					m.HistorySaveStatus = "Error: Project path not available."
				}
				// Go back to list after delete attempt
				m.CurrentScreen = app.ScreenProjectCommandsList
				m.SelectedProjectCommand = ""
				m.ProjectCommandsListIndex = 0
				m.ProjectCommandActionIndex = 0
				return m, nil
			case "Back":
				m.CurrentScreen = app.ScreenProjectCommandsList
				m.ProjectCommandActionIndex = 0
				return m, nil
			}

		case "esc", "b": // Go back to List
			m.CurrentScreen = app.ScreenProjectCommandsList
			m.ProjectCommandActionIndex = 0
			return m, nil
		}
	}

	// Default: return the model and no command if message type wasn't KeyMsg or key wasn't handled
	return m, nil
}

// ViewScreenProjectCommandActions renders the actions for a local project command.
func ViewScreenProjectCommandActions(m app.Model, registry *project.ProjectRegistry) string {
	// --- Restore Original Code ---
	header := app.TitleStyle.Render(fmt.Sprintf("Actions for Project Command: %s", m.SelectedProjectCommand)) + "\n"

	// Determine favorite status (with nil checks)
	favText := "Mark Favorite"
	if registry != nil && registry.FavoriteProjectCommands != nil {
		if _, isFav := registry.FavoriteProjectCommands[m.SelectedProjectCommand]; isFav {
			favText = "Unmark Favorite"
		}
	}
	actions := []string{"Run", favText, "Delete", "Back"}

	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Action:") + "\n\n")

	for i, action := range actions {
		if i == m.ProjectCommandActionIndex {
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

	return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, status, "\n", footer)
}
