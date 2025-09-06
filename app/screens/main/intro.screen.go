package mainScreen

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenSelect updates the "select" (login/offline) screen.
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

// ViewSelectScreen is the view for the "select" screen.
func ViewSelectScreen(m app.Model) string {
	title := app.TitleStyle.Render("=== Welcome ===")
	pathLine := app.PathStyle.Render(m.ProjectPath) // Gray path row
	body := title + "\n" + pathLine + "\n\n"

	// Optionally also show recognized packages or other info:
	body += sharedScreens.SummarizeProjectStats(m) + "\n"

	var loginOpt, offlineOpt string
	if m.IsLoggedIn {
		loginOpt = app.HighlightStyle.Render("> Login <")
		offlineOpt = app.ChoiceStyle.Render("Stay Offline")
	} else {
		loginOpt = app.ChoiceStyle.Render("Login")
		offlineOpt = app.HighlightStyle.Render("> Stay Offline <")
	}

	body += loginOpt + "\n" + offlineOpt + "\n\n"

	body += sharedScreens.Footer("↑↓ ←→ navigate", "ctrl+c quit")

	// Wrap the select screen content with a base container.
	panel := sharedScreens.BaseContainer(body)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, panel)
	}
	return panel
}
