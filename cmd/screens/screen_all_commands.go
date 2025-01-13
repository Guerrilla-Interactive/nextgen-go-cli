package screens

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenAll handles keypresses on the “all commands” screen.
func UpdateScreenAll(m myapp.model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "up", "k":
		if m.AllCmdsIndex > 0 {
			m.AllCmdsIndex--
		} else {
			m.AllCmdsIndex = m.AllCmdsTotal - 1
		}
	case "down", "j":
		if m.AllCmdsIndex < m.AllCmdsTotal-1 {
			m.AllCmdsIndex++
		} else {
			m.AllCmdsIndex = 0
		}
	case "enter":
		if m.AllCmdsIndex == m.AllCmdsTotal-1 {
			// “Back”
			m.CurrentScreen = myapp.screenMain
		} else {
			cmd := myapp.allCommands[m.AllCmdsIndex]
			recordCommand(&m, cmd)
		}
	}
	return m, nil
}

// ViewAllScreen renders the “all commands” screen.
func ViewAllScreen(m myapp.model) string {
	title := myapp.titleStyle.Render("=== All Commands ===")
	body := "\n\n" + myapp.subtitleStyle.Render("Select a command (Enter to log usage).") + "\n\n"

	for i, cmd := range myapp.allCommands {
		if i == m.AllCmdsIndex {
			body += myapp.highlightStyle.Render("> "+cmd+" <") + "\n"
		} else {
			body += myapp.choiceStyle.Render(cmd) + "\n"
		}
	}

	// “Back” item at the end
	if m.AllCmdsIndex == m.AllCmdsTotal-1 {
		body += myapp.highlightStyle.Render("> Back <") + "\n"
	} else {
		body += myapp.choiceStyle.Render("Back") + "\n"
	}

	body += "\n" + myapp.helpStyle.Render(
		"(Use up/down or j/k to move; Enter on 'Back' returns to main screen; q quits.)")

	return title + body
}
