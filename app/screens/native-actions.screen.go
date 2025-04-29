package screens

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenNativeActions handles navigation for the native command actions.
func UpdateScreenNativeActions(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	actions := []string{"Toggle Favorite", "Back"} // Only Favorite and Back for native commands
	numOptions := len(actions)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.NativeActionIndex = (m.NativeActionIndex + numOptions - 1) % numOptions

	case "down", "j":
		m.NativeActionIndex = (m.NativeActionIndex + 1) % numOptions

	case "enter":
		selectedAction := actions[m.NativeActionIndex]
		cmdName := m.SelectedNativeCommand

		switch selectedAction {
		case "Toggle Favorite":
			if registry != nil {
				if registry.FavoriteNativeCommands == nil {
					registry.FavoriteNativeCommands = make(map[string]bool)
				}
				// Toggle the favorite status
				if _, isFav := registry.FavoriteNativeCommands[cmdName]; isFav {
					delete(registry.FavoriteNativeCommands, cmdName)
				} else {
					registry.FavoriteNativeCommands[cmdName] = true
				}
				// Save the registry
				if err := registry.Save(); err != nil {
					fmt.Printf("Warning: Failed to save registry after toggling native favorite: %v\n", err)
				}
				// Go back to the list immediately after toggling
				m.CurrentScreen = app.ScreenNativeList
				m.NativeActionIndex = 0 // Reset action index
				return m, nil
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

	// Determine favorite status for display
	favText := "Mark Favorite"
	if registry != nil && registry.FavoriteNativeCommands != nil {
		if _, isFav := registry.FavoriteNativeCommands[m.SelectedNativeCommand]; isFav {
			favText = "Unmark Favorite"
		}
	}
	actions := []string{favText, "Back"}

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

	footer := app.HelpStyle.Render("Use ↑/↓ to navigate, Enter to select, Esc/b to go back.")

	return lipgloss.JoinVertical(lipgloss.Left, header, listPanel, "\n", footer)
}
