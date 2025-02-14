package app

import (
	"github.com/charmbracelet/lipgloss"
)

// Screen indicates which screen is currently shown.
// Typically, ScreenSelect is used for the intro/initial command selection,
// while ScreenMain displays the recent commands (this is now used as the starting screen).
type Screen int

const (
	ScreenSelect Screen = iota
	ScreenMain
	ScreenAll
	ScreenFilenamePrompt
	ScreenInstallDetails
)

// Model is the primary application state shared by all screens.
type Model struct {
	CurrentScreen Screen
	IsLoggedIn    bool
	SelectedIndex int
	AllCmdsIndex  int
	CreatedFiles  []string
	CursorVisible bool

	TotalItems   int
	AllCmdsTotal int
	ProjectPath  string
	// RecognizedPkgs holds detected package names.
	// With the advanced recognizer these are grouped (e.g. React frameworks are deduplicated
	// and multiple CSS frameworks are summarized) before display.
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

	// NEW: Field to hold the live file tree preview.
	LivePreview string

	// NEW: Field to track selected option on Install Details screen.
	InstallDetailsSelectedOption int

	// NEW: Used in the filename prompt screen to determine if the "[Back]" button is focused.
	PromptOptionFocused bool

	// NEW: Terminal dimensions (updated via tea.WindowSizeMsg)
	TerminalWidth  int
	TerminalHeight int
}

// Example styles (keep or remove as you prefer).
var (
	TitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	SubtitleStyle  = lipgloss.NewStyle().Bold(true)
	HighlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
	ChoiceStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	DocStyle       = lipgloss.NewStyle().Padding(1, 2).Margin(1, 2)
	HelpStyle      = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888"))
	PathStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	LinkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Underline(true)
)
