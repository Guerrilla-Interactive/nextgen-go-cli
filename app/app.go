package app

import (
	"github.com/charmbracelet/lipgloss"
)

// Screen indicates which screen is currently shown.
type Screen int

const (
	ScreenSelect Screen = iota
	ScreenMain
	ScreenAll
)

// Model is the primary application state shared by all screens.
type Model struct {
	CurrentScreen  Screen
	IsLoggedIn     bool
	SelectedIndex  int
	AllCmdsIndex   int
	TotalItems     int
	AllCmdsTotal   int
	ProjectPath    string
	RecognizedPkgs []string
}

// We can keep our shared “recentUsed”, “nextSteps”, and “allCommands” slices here:
var RecentUsed = []string{
	"add section",
	"remove section",
	"undo",
	"redo",
	"add page",
	"remove page",
	"add portable-component",
	"remove portable-component",
}

var NextSteps = []string{
	"Show all my commands",
	"LogoutOrLoginPlaceholder",
}

var AllCommands = []string{
	"ng add section",
	"ng remove section",
	"ng undo",
	"ng redo",
	"ng add page",
	"ng remove page",
	"ng add portable-component",
	"ng remove portable-component",
}

// Example styles (you may keep them here, or in a separate file):
var (
	TitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#5f00d7")).Padding(0, 1)
	SubtitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5f00d7"))
	HighlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
	ChoiceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	DocStyle       = lipgloss.NewStyle().Padding(1, 2).Margin(1, 2)
	HelpStyle      = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888"))
	PathStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)
