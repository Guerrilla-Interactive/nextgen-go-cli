package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Identifiers for the two screens in our little TUI
type screen int

const (
	screenSelect screen = iota
	screenMain
)

// Stylistic elements via Lip Gloss
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

// recentUsed starts with some defaults, but is mutable.
var recentUsed = []string{
	"ngc init",
	"ngc build",
	"ngc deploy",
	"ngc config set",
	"ngc help",
}

// nextSteps always has 2 items—first is “Show all my commands,” second is a placeholder
// that toggles between “Login” and “Logout.”  This second item is never directly displayed
// in the slice, but we fill it in at runtime below.
var nextSteps = []string{
	"Show all my commands",
	"LogoutOrLoginPlaceholder",
}

type model struct {
	currentScreen screen

	// Tracks if we're logged in or offline.
	isLoggedIn bool

	// selectedIndex: which item is highlighted (arrows up/down).
	selectedIndex int

	// totalItems = len(recentUsed) + len(nextSteps).
	totalItems int
}

func (m model) Init() tea.Cmd {
	return nil
}

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

func (m model) View() string {
	switch m.currentScreen {
	case screenSelect:
		return docStyle.Render(m.viewSelectScreen())
	case screenMain:
		return docStyle.Render(m.viewMainScreen())
	default:
		return docStyle.Render("Unknown screen\n")
	}
}

// updateScreenSelect: toggles between Login and Stay Offline
func (m model) updateScreenSelect(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "up", "k", "down", "j":
		// Flip the boolean each time up/down is pressed
		m.isLoggedIn = !m.isLoggedIn

	case "enter":
		// Move to main screen
		m.currentScreen = screenMain
	}
	return m
}

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
		itemName, isLast := m.getItemName(m.selectedIndex)
		if isLast {
			// The last item toggles login state
			m.isLoggedIn = !m.isLoggedIn
			m.currentScreen = screenSelect
		} else {
			// It's a command (either from recentUsed or nextSteps[0])
			m.recordCommand(itemName)
		}
	}
	return m
}

// getItemName returns the text of the item at the given index, and whether it's the last item.
func (m model) getItemName(index int) (string, bool) {
	offset := len(recentUsed) + (len(nextSteps) - 1) // last item index
	if index == offset {
		// This is the toggling item => “Login” or “Logout”
		if m.isLoggedIn {
			return "Logout", true
		}
		return "Login", true
	}

	// If index < len(recentUsed), it’s from recentUsed
	if index < len(recentUsed) {
		return recentUsed[index], false
	}

	// Else it's nextSteps[0] (the “Show all my commands”), since the last item is offset
	stepIndex := index - len(recentUsed)
	return nextSteps[stepIndex], false
}

/*
recordCommand moves the chosen command to the FRONT of recentUsed, removing duplicates.
We keep a maximum of e.g. 8 items in recentUsed.
Since we’re storing everything only in memory, if the user restarts the program,
the state is lost. But within a single run, we “remember” their selections.
*/
func (m *model) recordCommand(cmd string) {
	// Remove any existing instance of this command
	idx := -1
	for i, v := range recentUsed {
		if v == cmd {
			idx = i
			break
		}
	}
	if idx != -1 {
		recentUsed = append(recentUsed[:idx], recentUsed[idx+1:]...)
	}
	// Insert @ front
	recentUsed = append([]string{cmd}, recentUsed...)

	// Limit to 8 items max (feel free to adjust)
	if len(recentUsed) > 8 {
		recentUsed = recentUsed[:8]
	}

	// Recount total items
	m.totalItems = len(recentUsed) + len(nextSteps)
}

// viewSelectScreen: user picks “Login” or “Stay Offline”
func (m model) viewSelectScreen() string {
	title := titleStyle.Render("=== Welcome ===")
	body := "Use ↑/↓ (or j/k) to toggle between Login and Stay Offline, then press Enter.\n\n"

	var loginOpt, offlineOpt string
	if m.isLoggedIn {
		loginOpt = highlightStyle.Render("> Login <")
		offlineOpt = choiceStyle.Render("Stay Offline")
	} else {
		loginOpt = choiceStyle.Render("Login")
		offlineOpt = highlightStyle.Render("> Stay Offline <")
	}
	body += loginOpt + "\n" + offlineOpt + "\n\n"
	body += helpStyle.Render("(Press q to quit)")
	return title + "\n\n" + body
}

// viewMainScreen: show recent commands horizontally, then additional options
func (m model) viewMainScreen() string {
	titleText := "=== Offline Mode ==="
	if m.isLoggedIn {
		titleText = "=== Online Mode ==="
	}
	title := titleStyle.Render(titleText)

	body := "\n\n" + subtitleStyle.Render("Recent used commands (2 lines, up to 4 columns):") + "\n\n"

	// Show recentUsed in up-to-2 lines horizontally
	body += renderItemsHorizontally(recentUsed, &m, 0, 4)

	body += "\n" + subtitleStyle.Render("Additional Options:") + "\n\n"

	// We have 2 items in nextSteps:
	// [0] => "Show all my commands"
	// [1] => toggles between "Logout" and "Login"
	var finalItem string
	if m.isLoggedIn {
		finalItem = "Logout"
	} else {
		finalItem = "Login"
	}
	opts := []string{nextSteps[0], finalItem} // 2 items

	body += renderItemList(opts, &m, len(recentUsed))

	body += "\n" + helpStyle.Render(
		"(Use ↑/↓ or j/k to move, Enter on any command logs usage. "+
			"Enter on last item toggles login and returns to first screen, q quits.)")

	return title + body
}

// renderItemsHorizontally arranges items in up to “columns” columns per line.
// Here we specifically want 2 lines total, 4 columns per line for our 5+ possible items.
func renderItemsHorizontally(items []string, m *model, offset int, columns int) string {
	var outputLines []string
	var currentLine string

	for i, val := range items {
		// Start a new line every time we have multiples of columns
		if i != 0 && i%columns == 0 {
			outputLines = append(outputLines, currentLine)
			currentLine = ""
		}

		fullIndex := offset + i
		if m.selectedIndex == fullIndex {
			currentLine += highlightStyle.Render("> "+val+" <") + "  "
		} else {
			currentLine += choiceStyle.Render(val) + "  "
		}
	}

	// Append any leftover line
	if currentLine != "" {
		outputLines = append(outputLines, currentLine)
	}

	// Join lines
	var finalOutput string
	for _, line := range outputLines {
		finalOutput += line + "\n"
	}
	return finalOutput
}

// renderItemList enumerates items in a vertical list
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
	initialModel := model{
		currentScreen: screenSelect,
		isLoggedIn:    false,
		selectedIndex: 0,
		totalItems:    len(recentUsed) + len(nextSteps),
	}

	p := tea.NewProgram(initialModel)
	if err := p.Start(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
		os.Exit(1)
	}
}
