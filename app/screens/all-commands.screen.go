package screens

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenAll handles keypresses on the “all commands” screen.
func UpdateScreenAll(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
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
		// “Back”
		if m.AllCmdsIndex == m.AllCmdsTotal-1 {
			m.CurrentScreen = app.ScreenMain
		} else {
			cmd := app.AllCommands[m.AllCmdsIndex]
			recordCommand(&m, cmd)
		}
	}
	return m, nil
}

// ViewAllScreen renders the “all commands” screen.
func ViewAllScreen(m app.Model) string {
	title := app.TitleStyle.Render("=== All Commands ===")
	body := "\n\n" + app.SubtitleStyle.Render("Select a command (Enter to log usage).") + "\n\n"

	for i, cmd := range app.AllCommands {
		if i == m.AllCmdsIndex {
			body += app.HighlightStyle.Render("> "+cmd+" <") + "\n"
		} else {
			body += app.ChoiceStyle.Render(cmd) + "\n"
		}
	}

	// “Back” item at the end
	if m.AllCmdsIndex == m.AllCmdsTotal-1 {
		body += app.HighlightStyle.Render("> Back <") + "\n"
	} else {
		body += app.ChoiceStyle.Render("Back") + "\n"
	}

	body += "\n" + app.HelpStyle.Render("(Use up/down or j/k to move; Enter on 'Back' returns to main screen; q quits.)")

	return title + body
}
