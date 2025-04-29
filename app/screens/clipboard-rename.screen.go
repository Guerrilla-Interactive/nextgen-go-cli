package screens

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenRenameClipboard handles input for renaming a clipboard command.
func UpdateScreenRenameClipboard(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc", "b": // Go back to Actions screen
		m.CurrentScreen = app.ScreenClipboardActions
		// Keep m.SelectedClipboardCommand
		// Reset input and action index
		m.ClipboardRenameInput = ""
		m.ClipboardActionIndex = 0
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.ClipboardRenameInput)
		oldName := m.SelectedClipboardCommand

		if newName == "" || newName == oldName {
			// No change or empty name, just go back
			m.CurrentScreen = app.ScreenClipboardActions
			m.ClipboardRenameInput = ""
			m.ClipboardActionIndex = 0
			return m, nil
		}

		if registry != nil && registry.ClipboardCommands != nil {
			// Check if new name already exists
			if _, exists := registry.ClipboardCommands[newName]; exists {
				// TODO: Show an error message instead of just printing
				fmt.Printf("Error: Command name '%s' already exists.\n", newName)
				return m, nil // Stay on rename screen
			}

			// Get the spec with the old name
			if cmdSpec, ok := registry.ClipboardCommands[oldName]; ok {
				// Update the name within the spec
				cmdSpec.Name = newName
				// Delete the old entry
				delete(registry.ClipboardCommands, oldName)
				// Add the entry with the new name
				registry.ClipboardCommands[newName] = cmdSpec
				// Save the registry
				if err := registry.Save(); err != nil {
					fmt.Printf("Warning: Failed to save registry after renaming command: %v\n", err)
				}
			}
		}
		// Go back to the list screen after rename
		m.CurrentScreen = app.ScreenClipboardList
		m.ClipboardRenameInput = ""
		m.SelectedClipboardCommand = "" // Clear selection
		m.ClipboardListIndex = 0
		return m, nil

	// Handle text input
	default:
		if len(msg.String()) == 1 {
			m.ClipboardRenameInput += msg.String()
		} else if msg.String() == "backspace" && len(m.ClipboardRenameInput) > 0 {
			m.ClipboardRenameInput = m.ClipboardRenameInput[:len(m.ClipboardRenameInput)-1]
		}
	}

	return m, nil // Need cursor blink?
}

// ViewScreenRenameClipboard renders the rename input prompt.
func ViewScreenRenameClipboard(m app.Model) string {
	header := app.TitleStyle.Render(fmt.Sprintf("Rename: %s", m.SelectedClipboardCommand)) + "\n"

	prompt := fmt.Sprintf("Enter new name:\n> %s%s",
		m.ClipboardRenameInput,
		lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▎"), // Basic cursor
	)

	inputPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("15")).
		Render(prompt)

	footer := app.HelpStyle.Render("Enter to confirm, Esc/b to cancel.")

	return lipgloss.JoinVertical(lipgloss.Left, header, inputPanel, "\n", footer)
}
