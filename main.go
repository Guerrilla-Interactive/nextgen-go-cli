package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenSelect screen = iota
	screenMain
	screenAll
)

// --- Styles -----------------------------------------------------------------

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

	// New path style (50% grayish)
	pathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)

// --- Data -------------------------------------------------------------------

// We’ll look for these known packages in dependencies/devDependencies.
// Map key = the NPM package name, value = friendlier display label.
var knownPackages = map[string]string{
	"next":              "Next.js",
	"sanity":            "Sanity (CMS)",
	"tailwindcss":       "Tailwind CSS",
	"react-email":       "React Email",
	"styled-components": "styled-components",
	"gatsby":            "Gatsby",
	"contentful":        "Contentful",
	"strapi":            "Strapi",
	"vue":               "Vue.js",
	"react":             "React",
	"angular":           "Angular",
	// Add or remove as desired
}

// Minimal subset of fields from package.json
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// Our example commands
var recentUsed = []string{
	"add section",
	"remove section",
	"undo",
	"redo",
	"add page",
	"remove page",
	"add portable-component",
	"remove portable-component",
}

// Next-step items
var nextSteps = []string{
	"Show all my commands",
	"LogoutOrLoginPlaceholder",
}

// A larger command list shown on the “Show all” screen
var allCommands = []string{
	"ng add section",
	"ng remove section",
	"ng undo",
	"ng redo",
	"ng add page",
	"ng remove page",
	"ng add portable-component",
	"ng remove portable-component",
}

// --- Model ------------------------------------------------------------------

type model struct {
	currentScreen screen

	// Tracks if we're logged in or offline
	isLoggedIn bool

	// Indexes for selection
	selectedIndex int // On the main screen
	allCmdsIndex  int // On the “Show all commands” screen

	// Number of total items on main screen
	totalItems   int
	allCmdsTotal int

	// Project-related stats
	projectPath        string
	recognizedPackages []string // e.g. ["Next.js", "Tailwind CSS", ...]
}

// --- Init -------------------------------------------------------------------

// We load project info (including recognized packages) at startup.
func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		wd, _ := os.Getwd()
		recPkgs := detectFrameworks(wd)

		newM := m
		newM.projectPath = wd
		newM.recognizedPackages = recPkgs
		return newM
	}
}

// detectFrameworks reads package.json and sees if any known packages exist.
// Returns a slice of recognized packages in a more user-friendly form.
func detectFrameworks(projectPath string) []string {
	packageJSONPath := filepath.Join(projectPath, "package.json")
	data, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		// Couldn’t read package.json => no recognized packages
		return nil
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	foundSet := map[string]bool{}

	checkDep := func(deps map[string]string) {
		if deps == nil {
			return
		}
		for depName := range deps {
			if friendly, ok := knownPackages[depName]; ok {
				foundSet[friendly] = true
			}
		}
	}

	checkDep(pkg.Dependencies)
	checkDep(pkg.DevDependencies)

	// Convert map keys to a slice
	var results []string
	for pkgName := range foundSet {
		results = append(results, pkgName)
	}
	return results
}

// --- Update -----------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case model:
		// The updated model from Init()
		m.projectPath = typedMsg.projectPath
		m.recognizedPackages = typedMsg.recognizedPackages
		return m, nil

	case tea.KeyMsg:
		switch m.currentScreen {
		case screenSelect:
			m = m.updateScreenSelect(typedMsg)
		case screenMain:
			m = m.updateScreenMain(typedMsg)
		case screenAll:
			m = m.updateScreenAll(typedMsg)
		}
	}
	return m, nil
}

// --- View -------------------------------------------------------------------

func (m model) View() string {
	switch m.currentScreen {
	case screenSelect:
		return docStyle.Render(m.viewSelectScreen())
	case screenMain:
		return docStyle.Render(m.viewMainScreen())
	case screenAll:
		return docStyle.Render(m.viewAllScreen())
	default:
		return docStyle.Render("Unknown screen\n")
	}
}

// --- Screen Select (welcome/login/offline) ----------------------------------

func (m model) updateScreenSelect(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)
	case "up", "k", "down", "j":
		// Toggle online/offline
		m.isLoggedIn = !m.isLoggedIn
	case "enter":
		m.currentScreen = screenMain
	}
	return m
}

func (m model) viewSelectScreen() string {
	title := titleStyle.Render("=== Welcome ===")
	// First: show project info
	body := summarizeProjectStats(m) + "\n"

	// Show login/offline toggle
	var loginOpt, offlineOpt string
	if m.isLoggedIn {
		loginOpt = highlightStyle.Render("> Login <")
		offlineOpt = choiceStyle.Render("Stay Offline")
	} else {
		loginOpt = choiceStyle.Render("Login")
		offlineOpt = highlightStyle.Render("> Stay Offline <")
	}
	body += loginOpt + "\n" + offlineOpt + "\n\n"

	// Place the instructions at the bottom
	body += helpStyle.Render(
		"Use ↑/↓ (or j/k) to toggle between Login and Stay Offline, then press Enter.\n" +
			"(Press q to quit)")

	return title + "\n" + body
}

// --- Screen Main (recent commands + nextSteps) ------------------------------

func (m model) updateScreenMain(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)
	case "left", "h":
		if m.selectedIndex < len(recentUsed) && m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "right", "l":
		if m.selectedIndex < len(recentUsed)-1 {
			m.selectedIndex++
		}
	case "up", "k":
		m = m.handleUpInMain()
	case "down", "j":
		m = m.handleDownInMain()
	case "enter":
		itemName, isLast := m.getItemName(m.selectedIndex)
		if isLast {
			// Toggle login/offline
			m.isLoggedIn = !m.isLoggedIn
			m.currentScreen = screenSelect
		} else {
			// Possibly go to “Show all commands”
			if itemName == nextSteps[0] {
				m.currentScreen = screenAll
				m.allCmdsIndex = 0
				m.allCmdsTotal = len(allCommands) + 1
			} else {
				// Record the chosen command
				m.recordCommand(itemName)
			}
		}
	}
	return m
}

func (m model) handleUpInMain() model {
	if m.selectedIndex < len(recentUsed) {
		const columns = 4
		row := m.selectedIndex / columns
		if row == 0 {
			m.selectedIndex = m.totalItems - 1
		} else {
			col := m.selectedIndex % columns
			m.selectedIndex = (row-1)*columns + col
		}
	} else {
		stepIndex := m.selectedIndex - len(recentUsed)
		stepIndex--
		if stepIndex < 0 {
			rowCount := (len(recentUsed)-1)/4 + 1
			lastRow := rowCount - 1
			newIndex := lastRow * 4
			if newIndex >= len(recentUsed) {
				newIndex = len(recentUsed) - 1
			}
			m.selectedIndex = newIndex
		} else {
			m.selectedIndex = len(recentUsed) + stepIndex
		}
	}
	return m
}

func (m model) handleDownInMain() model {
	if m.selectedIndex < len(recentUsed) {
		const columns = 4
		row := m.selectedIndex / columns
		col := m.selectedIndex % columns
		nextRowIndex := (row+1)*columns + col
		if nextRowIndex < len(recentUsed) {
			m.selectedIndex = nextRowIndex
		} else {
			m.selectedIndex = len(recentUsed)
		}
	} else {
		stepIndex := m.selectedIndex - len(recentUsed)
		stepIndex++
		if stepIndex >= len(nextSteps) {
			m.selectedIndex = 0
		} else {
			m.selectedIndex = len(recentUsed) + stepIndex
		}
	}
	return m
}

func (m model) viewMainScreen() string {
	titleText := "=== Offline Mode ==="
	if m.isLoggedIn {
		titleText = "=== Online Mode ==="
	}
	title := titleStyle.Render(titleText)

	// Show project stats
	body := summarizeProjectStats(m) + "\n"

	// Recent commands
	body += subtitleStyle.Render("Recent used commands:") + "\n\n"
	body += renderItemsHorizontally(recentUsed, &m, 0, 4)

	// Additional options
	body += "\n"

	var finalItem string
	if m.isLoggedIn {
		finalItem = "Back"
	} else {
		finalItem = "Back"
	}
	opts := []string{nextSteps[0], finalItem}

	body += renderItemList(opts, &m, len(recentUsed))

	body += "\n" + helpStyle.Render(
		"(Use arrow keys or j/k/h/l to move; "+
			"q quits.)")

	return title + "\n" + body
}

// --- Screen All (larger commands list) --------------------------------------

func (m model) updateScreenAll(msg tea.KeyMsg) model {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)
	case "up", "k":
		if m.allCmdsIndex > 0 {
			m.allCmdsIndex--
		} else {
			m.allCmdsIndex = m.allCmdsTotal - 1
		}
	case "down", "j":
		if m.allCmdsIndex < m.allCmdsTotal-1 {
			m.allCmdsIndex++
		} else {
			m.allCmdsIndex = 0
		}
	case "enter":
		if m.allCmdsIndex == m.allCmdsTotal-1 {
			// “Back”
			m.currentScreen = screenMain
		} else {
			cmd := allCommands[m.allCmdsIndex]
			m.recordCommand(cmd)
		}
	}
	return m
}

func (m model) viewAllScreen() string {
	title := titleStyle.Render("=== All Commands ===")
	body := "\n\n" + subtitleStyle.Render("Select a command (Enter to log usage).") + "\n\n"

	for i, cmd := range allCommands {
		if i == m.allCmdsIndex {
			body += highlightStyle.Render("> "+cmd+" <") + "\n"
		} else {
			body += choiceStyle.Render(cmd) + "\n"
		}
	}
	// “Back” item
	if m.allCmdsIndex == m.allCmdsTotal-1 {
		body += highlightStyle.Render("> Back <") + "\n"
	} else {
		body += choiceStyle.Render("Back") + "\n"
	}

	body += "\n" + helpStyle.Render(
		"(Use up/down or j/k to move; Enter on 'Back' returns to main screen; q quits.)")

	return title + body
}

// --- Helpers ----------------------------------------------------------------

// Summarize the project path and recognized packages in a neat block
func summarizeProjectStats(m model) string {
	stats := pathStyle.Render(m.projectPath) + "\n\n"

	if len(m.recognizedPackages) == 0 {
		stats += "    • None recognized packages\n"
	} else {
		// Show recognized packages horizontally, up to 6 columns, 2 lines (12 max)
		// If more than 12 recognized, we truncate for this demo.
		truncated := m.recognizedPackages
		if len(truncated) > 12 {
			truncated = truncated[:12]
		}

		stats += renderPackagesHorizontally(truncated, 6)
	}
	return stats
}

// renderPackagesHorizontally arranges items in up to 'columns' columns per line.
// If there are more items than 2 lines * columns, we just won't see them if we truncated above.
func renderPackagesHorizontally(pkgs []string, columns int) string {
	if len(pkgs) == 0 {
		return "    • None recognized packages\n"
	}

	var lines []string
	var currentLine []string

	for i, pkg := range pkgs {
		currentLine = append(currentLine, pkg)
		// If we hit 'columns' items, we push the line and reset
		if (i+1)%columns == 0 {
			lines = append(lines, strings.Join(currentLine, " | "))
			currentLine = nil
		}
	}
	// Append any leftover
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " | "))
	}

	var result string
	for _, line := range lines {
		result += "" + line + "\n"
	}
	return result
}

func (m model) getItemName(index int) (string, bool) {
	offset := len(recentUsed) + (len(nextSteps) - 1)
	if index == offset {
		if m.isLoggedIn {
			return "Logout", true
		}
		return "Login", true
	}
	if index < len(recentUsed) {
		return recentUsed[index], false
	}
	stepIndex := index - len(recentUsed)
	return nextSteps[stepIndex], false
}

// Move the chosen command to the front of recentUsed, removing duplicates, limit to 8.
func (m *model) recordCommand(cmd string) {
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
	recentUsed = append([]string{cmd}, recentUsed...)

	if len(recentUsed) > 8 {
		recentUsed = recentUsed[:8]
	}

	m.totalItems = len(recentUsed) + len(nextSteps)
}

// Render items horizontally in rows of “columns” columns (for the main screen’s recentUsed).
func renderItemsHorizontally(items []string, m *model, offset int, columns int) string {
	var outputLines []string
	var currentLine string

	for i, val := range items {
		if i != 0 && i%columns == 0 {
			outputLines = append(outputLines, currentLine)
			currentLine = ""
		}

		fullIndex := offset + i
		if m.selectedIndex == fullIndex && m.currentScreen == screenMain {
			currentLine += highlightStyle.Render("> "+val+" <") + "  "
		} else {
			currentLine += choiceStyle.Render(val) + "  "
		}
	}
	if currentLine != "" {
		outputLines = append(outputLines, currentLine)
	}

	var finalOutput string
	for _, line := range outputLines {
		finalOutput += line + "\n"
	}
	return finalOutput
}

// Renders a vertical list (for nextSteps on main screen).
func renderItemList(items []string, m *model, offset int) string {
	var output string
	for i, val := range items {
		fullIndex := offset + i
		if m.selectedIndex == fullIndex && m.currentScreen == screenMain {
			output += "" + highlightStyle.Render("> "+val+" <") + "\n"
		} else {
			output += "" + choiceStyle.Render(val) + "\n"
		}
	}
	return output
}

// --- Main -------------------------------------------------------------------

func main() {
	initialModel := model{
		currentScreen: screenSelect,
		isLoggedIn:    false,
		selectedIndex: 0,
		allCmdsIndex:  0,

		totalItems:   len(recentUsed) + len(nextSteps),
		allCmdsTotal: len(allCommands) + 1, // +1 for "Back"

		projectPath:        "",
		recognizedPackages: nil,
	}

	p := tea.NewProgram(initialModel)
	if err := p.Start(); err != nil {
		log.Fatalf("Error running TUI: %v", err)
		os.Exit(1)
	}
}
