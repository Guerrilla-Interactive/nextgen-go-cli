package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenClipboardActions handles navigation for the command actions.
func UpdateScreenClipboardActions(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	actions := []string{"Toggle Favorite", "Rename", "Save to Project", "Delete", "Back"}
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
		case "Toggle Favorite":
			if registry != nil && registry.ClipboardCommands != nil {
				if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok {
					cmdSpec.IsFavorite = !cmdSpec.IsFavorite
					registry.ClipboardCommands[cmdName] = cmdSpec
					if err := registry.Save(); err != nil {
						fmt.Printf("Warning: Failed to save registry after toggling favorite: %v\n", err)
					}
					// Go back to the list immediately after toggling
					m.CurrentScreen = app.ScreenClipboardList
					m.ClipboardActionIndex = 0 // Reset action index
					return m, nil
				}
			}
		case "Rename":
			// Navigate to Rename screen (Task #50)
			m.CurrentScreen = app.ScreenRenameClipboard
			m.ClipboardRenameInput = cmdName // Pre-fill with current name
			// TODO: Add state/logic for rename screen
			return m, nil
		case "Save to Project":
			if registry != nil && m.ProjectPath != "" {
				if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok {
					localCmdDir := filepath.Join(m.ProjectPath, ".nextgen", "local-commands")
					if err := os.MkdirAll(localCmdDir, 0755); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Error creating dir: %v", err)
					} else {
						// Use kebab-case for filename, ensure .json extension
						fileName := commands.ToKebabCase(cmdName) + ".json"
						targetPath := filepath.Join(localCmdDir, fileName)
						if err := os.WriteFile(targetPath, []byte(cmdSpec.Template), 0644); err != nil {
							m.HistorySaveStatus = fmt.Sprintf("Error saving file: %v", err)
						} else {
							m.HistorySaveStatus = fmt.Sprintf("Saved to project: %s", fileName)
							// --- If save successful, remove from global clipboard list ---
							delete(registry.ClipboardCommands, cmdName)
							// --- End removal ---
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
			// Go back to the list screen after attempting save
			m.CurrentScreen = app.ScreenClipboardList
			m.ClipboardActionIndex = 0
			return m, nil
		case "Delete":
			if registry != nil && registry.ClipboardCommands != nil {
				delete(registry.ClipboardCommands, cmdName)
				if err := registry.Save(); err != nil {
					fmt.Printf("Warning: Failed to save registry after deleting command: %v\n", err)
				}
				// Go back to the list screen after deleting
				m.CurrentScreen = app.ScreenClipboardList
				m.SelectedClipboardCommand = "" // Clear selection
				m.ClipboardListIndex = 0        // Reset list index
				m.ClipboardActionIndex = 0
				return m, nil
			}
		case "Back":
			m.CurrentScreen = app.ScreenClipboardList
			m.ClipboardActionIndex = 0
			return m, nil
		}

	case "esc", "b": // Go back to List
		m.CurrentScreen = app.ScreenClipboardList
		m.ClipboardActionIndex = 0
		return m, nil
	}

	return m, nil
}

// ViewScreenClipboardActions renders the actions available for a selected clipboard command.
func ViewScreenClipboardActions(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render(fmt.Sprintf("Actions for: %s", m.SelectedClipboardCommand)) + "\n"

	// Check if command still exists (might have been deleted)
	cmdSpec, exists := registry.ClipboardCommands[m.SelectedClipboardCommand]
	if registry == nil || !exists {
		content := app.ChoiceStyle.Render("Selected command no longer exists. Press Esc/b to go back.")
		return lipgloss.JoinVertical(lipgloss.Left, header, content)
	}

	// Determine favorite status for display
	favText := "Mark Favorite"
	if cmdSpec.IsFavorite {
		favText = "Unmark Favorite"
	}
	actions := []string{favText, "Rename", "Save to Project", "Delete", "Back"}

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

	footer := app.HelpStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc/b to go back.")

	return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, "\n", footer)
}
