package screens

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenMain handles input for the main screen.
func UpdateScreenMain(m myapp.model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)
	case "left", "h":
		if m.SelectedIndex < len(myapp.recentUsed) && m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
	case "right", "l":
		if m.SelectedIndex < len(myapp.recentUsed)-1 {
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
			m.CurrentScreen = myapp.screenSelect
		} else {
			// Possibly go to “Show all commands”
			if itemName == myapp.nextSteps[0] {
				m.CurrentScreen = myapp.screenAll
				m.AllCmdsIndex = 0
				m.AllCmdsTotal = len(myapp.allCommands) + 1
			} else {
				recordCommand(&m, itemName)
			}
		}
	}
	return m, nil
}

// ViewMainScreen is the view for the main screen.
func ViewMainScreen(m myapp.model) string {
	titleText := "=== Offline Mode ==="
	if m.IsLoggedIn {
		titleText = "=== Online Mode ==="
	}
	title := myapp.titleStyle.Render(titleText)

	body := summarizeProjectStats(m) + "\n"

	body += myapp.subtitleStyle.Render("Recent used commands:") + "\n\n"
	body += renderItemsHorizontally(myapp.recentUsed, &m, 0, 4)

	body += "\n"

	// nextSteps: [ "Show all my commands", "LogoutOrLoginPlaceholder" ]
	// We'll rename "LogoutOrLoginPlaceholder" to "Back" for the user:
	opts := []string{myapp.nextSteps[0], "Back"}
	body += renderItemList(opts, &m, len(myapp.recentUsed))

	body += "\n" + myapp.helpStyle.Render(
		"(Use arrow keys or j/k/h/l to move; q quits.)")

	return title + "\n" + body
}

// handleUpInMain moves the selection “up” in the grid or list.
func handleUpInMain(m myapp.model) myapp.model {
	if m.SelectedIndex < len(myapp.recentUsed) {
		const columns = 4
		row := m.SelectedIndex / columns
		if row == 0 {
			m.SelectedIndex = m.TotalItems - 1
		} else {
			col := m.SelectedIndex % columns
			m.SelectedIndex = (row-1)*columns + col
		}
	} else {
		stepIndex := m.SelectedIndex - len(myapp.recentUsed)
		stepIndex--
		if stepIndex < 0 {
			// Wrap up to the bottom row of recentUsed
			rowCount := (len(myapp.recentUsed)-1)/4 + 1
			lastRow := rowCount - 1
			newIndex := lastRow * 4
			if newIndex >= len(myapp.recentUsed) {
				newIndex = len(myapp.recentUsed) - 1
			}
			m.SelectedIndex = newIndex
		} else {
			m.SelectedIndex = len(myapp.recentUsed) + stepIndex
		}
	}
	return m
}

// handleDownInMain moves the selection “down” in the grid or list.
func handleDownInMain(m myapp.model) myapp.model {
	if m.SelectedIndex < len(myapp.recentUsed) {
		const columns = 4
		row := m.SelectedIndex / columns
		col := m.SelectedIndex % columns
		nextRowIndex := (row+1)*columns + col
		if nextRowIndex < len(myapp.recentUsed) {
			m.SelectedIndex = nextRowIndex
		} else {
			m.SelectedIndex = len(myapp.recentUsed)
		}
	} else {
		stepIndex := m.SelectedIndex - len(myapp.recentUsed)
		stepIndex++
		if stepIndex >= len(myapp.nextSteps) {
			m.SelectedIndex = 0
		} else {
			m.SelectedIndex = len(myapp.recentUsed) + stepIndex
		}
	}
	return m
}
