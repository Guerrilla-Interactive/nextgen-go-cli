package clipboard

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenRenameClipboard handles input for renaming a clipboard command.
// It now uses the textinput bubble for managing input.
func UpdateScreenRenameClipboard(m app.Model, msg tea.Msg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc: // Treat Esc like Ctrl+C for cancelling rename
			m.ClipboardRenameInput.Blur()
			m.ClipboardRenameInput.SetValue("") // Clear input on cancel
			m.CurrentScreen = app.ScreenClipboardActions
			// Keep m.SelectedClipboardCommand
			m.ClipboardActionIndex = 0
			return m, nil

		case tea.KeyEnter:
			newName := strings.TrimSpace(m.ClipboardRenameInput.Value())
			oldName := m.SelectedClipboardCommand

			// Don't rename if empty or same name
			if newName == "" || newName == oldName {
				m.ClipboardRenameInput.Blur()
				m.ClipboardRenameInput.SetValue("")
				m.CurrentScreen = app.ScreenClipboardList // Go back to list
				m.ClipboardActionIndex = 0
				m.SelectedClipboardCommand = "" // Clear selection as action is done
				m.ClipboardListIndex = 0
				return m, nil
			}

			if registry != nil && registry.ClipboardCommands != nil {
				// Check if new name already exists
				if _, exists := registry.ClipboardCommands[newName]; exists {
					// TODO: Show a proper error message in the UI instead of printing
					m.HistorySaveStatus = fmt.Sprintf("Error: Command name '%s' already exists.", newName)
					// Stay on rename screen, allow user to correct
					return m, nil
				}

				// Proceed with rename
				if cmdSpec, ok := registry.ClipboardCommands[oldName]; ok {
					cmdSpec.Name = newName                        // Update name in spec
					delete(registry.ClipboardCommands, oldName)   // Delete old entry
					registry.ClipboardCommands[newName] = cmdSpec // Add new entry
					if err := registry.Save(); err != nil {
						// TODO: Show error in UI
						m.HistorySaveStatus = fmt.Sprintf("Warning: Failed to save registry: %v", err)
					} else {
						m.HistorySaveStatus = fmt.Sprintf("Renamed '%s' to '%s'", oldName, newName)
					}
				} else {
					m.HistorySaveStatus = fmt.Sprintf("Error: Could not find original command '%s'", oldName)
				}
			} else {
				m.HistorySaveStatus = "Error: Registry unavailable for rename."
			}

			// Rename complete (or failed), go back to the list screen
			m.ClipboardRenameInput.Blur()
			m.ClipboardRenameInput.SetValue("")
			m.CurrentScreen = app.ScreenClipboardList
			m.SelectedClipboardCommand = "" // Clear selection
			m.ClipboardListIndex = 0
			m.ClipboardActionIndex = 0 // Reset action index too
			return m, nil

		// Default case for KeyMsg handles text input via the textinput bubble
		default:
			// Let the textinput bubble handle the key press
			m.ClipboardRenameInput, cmd = m.ClipboardRenameInput.Update(msg)
			return m, cmd
		}
	}

	// Handle non-key messages if necessary, otherwise pass to textinput
	m.ClipboardRenameInput, cmd = m.ClipboardRenameInput.Update(msg)
	return m, cmd
}

// ViewScreenRenameClipboard renders the rename input prompt using the textinput bubble.
func ViewScreenRenameClipboard(m app.Model) string {
	header := app.TitleStyle.Render(fmt.Sprintf("Rename: %s", m.SelectedClipboardCommand)) + "\n"

	prompt := "Enter new name:"

	// Use the textinput's View method
	inputView := m.ClipboardRenameInput.View()

	// Combine elements vertically
	view := lipgloss.JoinVertical(lipgloss.Left,
		prompt,
		inputView, // Render the text input bubble
	)

	inputPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("15")).
		Render(view)

	// Show status message if any
	status := ""
	if m.HistorySaveStatus != "" {
		status = app.ChoiceStyle.Render(m.HistorySaveStatus) // Reuse ChoiceStyle or define an ErrorStyle
	}

	footer := app.HelpStyle.Render("Enter to confirm, Esc to cancel.")

	// Combine all parts
	return lipgloss.JoinVertical(lipgloss.Left, header, inputPanel, status, "\n", footer)
}
