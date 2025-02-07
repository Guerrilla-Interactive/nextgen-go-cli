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
	ScreenFilenamePrompt
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
	TempFilename   string // Used for single-variable input.
	PendingCommand string // Stores the command that triggered the prompt.

	// Fields for multi-variable mode:
	MultipleVariables    bool              // True when the command requires multiple variables.
	VariableKeys         []string          // List of keys (e.g. ["Component", "Page", "Feature"]).
	CurrentVariableIndex int               // Index for tracking which variable is being collected.
	Variables            map[string]string // Map to store the user's input for each variable.

	// List of files we plan to generate (for the FileGenModel):
	PlannedFiles []string

	// Add a field to hold the FileGenModel
}

// Example styles (keep or remove as you prefer).
var (
	TitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(2)
	SubtitleStyle  = lipgloss.NewStyle().Bold(true)
	HighlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
	ChoiceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	DocStyle       = lipgloss.NewStyle().Padding(1, 2).Margin(1, 2)
	HelpStyle      = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888"))
	PathStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
)
