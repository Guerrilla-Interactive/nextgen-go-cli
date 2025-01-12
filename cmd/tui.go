package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenSelect screen = iota
	screenMain
)

// Lip Gloss styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5f00d7")).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5f00d7"))

	highlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFA500"))

	choiceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	docStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Margin(1, 2)

	helpStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#888888"))
)

// The user’s main menu items are split into two sections:
// “Recent used commands” and “Additional Options.”  We’ll display
// them all on a single screen but label them separately.
var recentUsed = []string{
	"ngc init",
	"ngc build",
	"ngc deploy",
	"ngc config set",
	"ngc help",
}

var nextSteps = []string{
	"Show all my commands",
	"Find Command",
	// This will toggle depending on whether we’re actually logged in or not.
	// We’ll display either “Logout” or “Login” for this item.
	// The code below will handle the correct text at runtime.
	"LogoutOrLoginPlaceholder",
}

// Model holds the UI state
type model struct {
	currentScreen screen

	// Are we logged in (true) or offline/logged out (false)?
	isLoggedIn bool

	// Which item is currently highlighted (for arrow navigation) on the main screen
	selectedIndex int

	// Total count of items on the main screen
	totalItems int
}

// Init implements tea.Model
func (m model) Init() tea.Cmd {
	return nil
}

// Update processes incoming events
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.KeyMsg:
		switch m.currentScreen {
		case screenSelect:
			m = m.updateScreenSelect(typedMsg)
		case screenMain:
			m = m.updateScreenMain(typedMsg)
		}
	}
	return m, nil
}

// View is responsible for rendering the current UI
func (m model) View() string {
	var s string
	switch m.currentScreen {
	case screenSelect:
		s = m.viewSelectScreen()
	case screenMain:
		s = m.viewMainScreen()
	}
	return docStyle.Render(s)
}

// updateScreenSelect handles key presses on the initial screen
func (m model) updateScreenSelect(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		// Quit
		os.Exit(0)

	case "up", "k", "down", "j":
		// Toggle between "Login" and "Stay Offline"
		m.isLoggedIn = !m.isLoggedIn

	case "enter":
		// Proceed to the main screen
		m.currentScreen = screenMain
	}
	return m
}

// updateScreenMain navigates the list on the main screen
func (m model) updateScreenMain(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "up", "k":
		m.selectedIndex--
		if m.selectedIndex < 0 {
			m.selectedIndex = m.totalItems - 1
		}

	case "down", "j":
		m.selectedIndex++
		if m.selectedIndex >= m.totalItems {
			m.selectedIndex = 0
		}

	case "enter":
		// If the user selects the last item — which is "Login/Logout" — toggle isLoggedIn
		offset := len(recentUsed) + (len(nextSteps) - 1)
		if m.selectedIndex == offset {
			// Toggle login state
			m.isLoggedIn = !m.isLoggedIn
		}
	}
	return m
}

// viewSelectScreen shows two visible choices: "Login" or "Stay Offline"
func (m model) viewSelectScreen() string {
	title := titleStyle.Render("=== Welcome ===")
	body := "Use ↑/↓ (or j/k) to toggle between Login and Stay Offline, then press Enter.\n\n"

	var loginOption, offlineOption string
	// If isLoggedIn is true => highlight "Login," else highlight "Stay Offline"
	if m.isLoggedIn {
		loginOption = highlightStyle.Render("> Login <")
		offlineOption = choiceStyle.Render("Stay Offline")
	} else {
		loginOption = choiceStyle.Render("Login")
		offlineOption = highlightStyle.Render("> Stay Offline <")
	}

	body += loginOption + "\n" + offlineOption + "\n\n"
	body += helpStyle.Render("(Press q to quit.)")
	return title + "\n\n" + body
}

// viewMainScreen merges the old “login” and “offline” screens into one.
// The only difference: if isLoggedIn is false, we display “Login” for
// the last item. If isLoggedIn is true, we display “Logout.”
func (m model) viewMainScreen() string {
	// Title
	var screenTitle string
	if m.isLoggedIn {
		screenTitle = "=== Online Mode ==="
	} else {
		screenTitle = "=== Offline Mode ==="
	}
	title := titleStyle.Render(screenTitle)

	// We display two sections: "Recent used commands" and "Additional Options."
	// The last item in "Additional Options" is "Login" or "Logout," depending on isLoggedIn.
	body := "\n\n"

	// "Recent used commands" heading
	body += subtitleStyle.Render("Recent used commands:") + "\n\n"

	allItems := make([]string, 0)
	for _, r := range recentUsed {
		allItems = append(allItems, r)
	}

	// "Additional Options" heading
	body += renderItemList(allItems, &m, 0)

	body += "\n" + subtitleStyle.Render("Additional Options:") + "\n\n"

	optItems := make([]string, 0)
	optItems = append(optItems, nextSteps[0]) // "Show all my commands"
	optItems = append(optItems, nextSteps[1]) // "Find Command"

	// The last item toggles between "Logout" and "Login" based on isLoggedIn
	var finalItem string
	if m.isLoggedIn {
		finalItem = "Logout"
	} else {
		finalItem = "Login"
	}
	optItems = append(optItems, finalItem)

	// Build a single list with offset indexing
	// The offset for options is len(recentUsed)
	body += renderItemList(optItems, &m, len(recentUsed))

	body += "\n" + helpStyle.Render("(Use ↑/↓ or j/k to move • Enter on the last item toggles Login/Logout • q to quit)")
	return title + body
}

// renderItemList is a helper function that prints the given items with highlight if selected
func renderItemList(items []string, m *model, offset int) string {
	var output string
	for i, val := range items {
		fullIndex := offset + i
		if m.selectedIndex == fullIndex {
			output += "  " + highlightStyle.Render("> "+val+" <") + "\n"
		} else {
			output += "  " + choiceStyle.Render(val) + "\n"
		}
	}
	return output
}

func main() {
	// We have len(recentUsed) + len(nextSteps) total items in the main menu,
	// but recall we’re dynamically adjusting the last item (“Logout” vs. “Login”)
	// in code. So total is simply the sum of everything.
	totalItems := len(recentUsed) + len(nextSteps)

	m := model{
		currentScreen: screenSelect,
		isLoggedIn:    false, // user starts out “logged off”
		selectedIndex: 0,
		totalItems:    totalItems,
	}

	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
		os.Exit(1)
	}
}
