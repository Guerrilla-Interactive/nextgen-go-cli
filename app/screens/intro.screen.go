package screens

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenSelect updates the “select” (login/offline) screen.
func UpdateScreenSelect(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "up", "k", "down", "j":
		// Toggle online/offline
		m.IsLoggedIn = !m.IsLoggedIn

	case "enter":
		// Move on to the main screen
		m.CurrentScreen = app.ScreenMain
	}
	return m, nil
}

// ViewSelectScreen is the view for the “select” screen.
func ViewSelectScreen(m app.Model) string {
	title := app.TitleStyle.Render("=== Welcome ===")
	pathLine := app.PathStyle.Render(m.ProjectPath) // Gray path row
	body := title + "\n" + pathLine + "\n\n"

	// Optionally also show recognized packages or other info:
	body += summarizeProjectStats(m) + "\n"

	var loginOpt, offlineOpt string
	if m.IsLoggedIn {
		loginOpt = app.HighlightStyle.Render("> Login <")
		offlineOpt = app.ChoiceStyle.Render("Stay Offline")
	} else {
		loginOpt = app.ChoiceStyle.Render("Login")
		offlineOpt = app.HighlightStyle.Render("> Stay Offline <")
	}

	body += loginOpt + "\n" + offlineOpt + "\n\n"

	body += app.HelpStyle.Render(
		"Use ↑/↓ (or j/k) to toggle between Login and Stay Offline, then press Enter.\n" +
			"(Press q to quit)")

	return body
}
