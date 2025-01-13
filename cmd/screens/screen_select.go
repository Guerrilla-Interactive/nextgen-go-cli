package screens

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	// <-- Import path for your main package if needed
)

// UpdateScreenSelect updates the “select” (welcome/offline/login) screen.
func UpdateScreenSelect(m myapp.model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "up", "k", "down", "j":
		// Toggle online/offline
		m.IsLoggedIn = !m.IsLoggedIn

	case "enter":
		// Move on to the main screen
		m.CurrentScreen = myapp.screenMain
	}
	return m, nil
}

// ViewSelectScreen is the view for the “select” screen.
func ViewSelectScreen(m myapp.model) string {
	title := myapp.titleStyle.Render("=== Welcome ===")
	body := summarizeProjectStats(m) + "\n"

	var loginOpt, offlineOpt string
	if m.IsLoggedIn {
		loginOpt = myapp.highlightStyle.Render("> Login <")
		offlineOpt = myapp.choiceStyle.Render("Stay Offline")
	} else {
		loginOpt = myapp.choiceStyle.Render("Login")
		offlineOpt = myapp.highlightStyle.Render("> Stay Offline <")
	}

	body += loginOpt + "\n" + offlineOpt + "\n\n"

	body += myapp.helpStyle.Render(
		"Use ↑/↓ (or j/k) to toggle between Login and Stay Offline, then press Enter.\n" +
			"(Press q to quit)")

	return title + "\n" + body
}
