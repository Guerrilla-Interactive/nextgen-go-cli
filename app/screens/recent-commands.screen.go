package screens

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenMain handles input for the main screen.
func UpdateScreenMain(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "left", "h":
		if m.SelectedIndex < len(app.RecentUsed) && m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
	case "right", "l":
		if m.SelectedIndex < len(app.RecentUsed)-1 {
			m.SelectedIndex++
		}

	case "up", "k":
		m = handleUpInMain(m)
	case "down", "j":
		m = handleDownInMain(m)

	case "enter":
		itemName, isLast := getItemName(m, m.SelectedIndex)
		if isLast {
			// Toggle login/offline and go back to select screen
			m.IsLoggedIn = !m.IsLoggedIn
			m.CurrentScreen = app.ScreenSelect
		} else {
			// Possibly go to “Show all commands”
			if itemName == app.NextSteps[0] {
				m.CurrentScreen = app.ScreenAll
				m.AllCmdsIndex = 0
				m.AllCmdsTotal = len(app.AllCommands) + 1
			} else {
				recordCommand(&m, itemName)
			}
		}
	}
	return m, nil
}

// ViewMainScreen is the view for the main screen.
func ViewMainScreen(m app.Model) string {
	titleText := "=== Offline Mode ==="
	if m.IsLoggedIn {
		titleText = "=== Online Mode ==="
	}
	title := app.TitleStyle.Render(titleText)

	body := summarizeProjectStats(m) + "\n"

	body += app.SubtitleStyle.Render("Recent used commands:") + "\n\n"
	body += renderItemsHorizontally(app.RecentUsed, &m, 0, 4)

	body += "\n"

	// nextSteps: [ "Show all my commands", "LogoutOrLoginPlaceholder" ]
	// We'll rename the second item to "Back" for the user:
	opts := []string{app.NextSteps[0], "Back"}
	body += renderItemList(opts, &m, len(app.RecentUsed))

	body += "\n" + app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)")

	return title + "\n" + body
}

func handleUpInMain(m app.Model) app.Model {
	if m.SelectedIndex < len(app.RecentUsed) {
		const columns = 4
		row := m.SelectedIndex / columns
		if row == 0 {
			m.SelectedIndex = m.TotalItems - 1
		} else {
			col := m.SelectedIndex % columns
			m.SelectedIndex = (row-1)*columns + col
		}
	} else {
		stepIndex := m.SelectedIndex - len(app.RecentUsed)
		stepIndex--
		if stepIndex < 0 {
			// Wrap up to the bottom row
			rowCount := (len(app.RecentUsed)-1)/4 + 1
			lastRow := rowCount - 1
			newIndex := lastRow * 4
			if newIndex >= len(app.RecentUsed) {
				newIndex = len(app.RecentUsed) - 1
			}
			m.SelectedIndex = newIndex
		} else {
			m.SelectedIndex = len(app.RecentUsed) + stepIndex
		}
	}
	return m
}

func handleDownInMain(m app.Model) app.Model {
	if m.SelectedIndex < len(app.RecentUsed) {
		const columns = 4
		row := m.SelectedIndex / columns
		col := m.SelectedIndex % columns
		nextRowIndex := (row+1)*columns + col
		if nextRowIndex < len(app.RecentUsed) {
			m.SelectedIndex = nextRowIndex
		} else {
			m.SelectedIndex = len(app.RecentUsed)
		}
	} else {
		stepIndex := m.SelectedIndex - len(app.RecentUsed)
		stepIndex++
		if stepIndex >= len(app.NextSteps) {
			// Wrap
			m.SelectedIndex = 0
		} else {
			m.SelectedIndex = len(app.RecentUsed) + stepIndex
		}
	}
	return m
}
