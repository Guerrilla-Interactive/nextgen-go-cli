package app

import (
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/textinput"
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
	ScreenProjectStats
	ScreenCommandHistory
	ScreenCommandsCategory
	ScreenClipboardList
	ScreenClipboardActions
	ScreenRenameClipboard
	ScreenNativeList
	ScreenNativeActions
	ScreenProjectCommandsList
	ScreenProjectCommandActions
)

// Model is the primary application state shared by all screens.
type Model struct {
	CurrentScreen Screen
	IsLoggedIn    bool

	// --- Navigation/Selection State ---
	MainScreenFocus              string // "action" or "list"
	ActionIndex                  int    // Index for the top action bar
	SelectedIndex                int    // Index for the main command list (relative to current page)
	AllCmdsIndex                 int    // Index for the 'all commands' screen
	StatsScreenIndex             int
	HistoryScreenIndex           int
	CommandsCategoryIndex        int
	ClipboardListIndex           int
	ClipboardActionIndex         int
	NativeListIndex              int
	NativeActionIndex            int
	ProjectCommandsListIndex     int
	ProjectCommandActionIndex    int
	InstallDetailsSelectedOption int
	PromptOptionFocused          bool
	LastActionIndex              int // Keep for remembering last action focus?

	// ... (Rest of the fields: CreatedFiles, Preview fields, Variables, Paginators, etc.)
	CreatedFiles  []string
	CursorVisible bool
	AllCmdsTotal  int
	ProjectPath   string
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

	// Preview fields:
	CurrentPreviewType string // "file-tree", "stats", or "none"
	FileTreePreview    string // Holds the generated file tree preview string.
	StatsPreview       string // Holds the generated project stats preview string.

	// NEW: Terminal dimensions (updated via tea.WindowSizeMsg)
	TerminalWidth  int
	TerminalHeight int

	// NEW: Application Version (passed from main)
	Version string

	// NEW: Status message for debugging history saving
	HistorySaveStatus string

	// NEW: State for Command History screen
	HistoryFileTreePreview string

	// NEW: State for Native/Clipboard List Previews
	NativeListPreview    string
	ClipboardListPreview string

	// Paginator state
	ClipboardPaginator       paginator.Model
	NativePaginator          paginator.Model
	ProjectCommandsPaginator paginator.Model
	MainListPaginator        paginator.Model

	// --- Data & Other State ---
	SelectedClipboardCommand string
	ClipboardRenameInput     textinput.Model
	SelectedNativeCommand    string
	SelectedProjectCommand   string
	ProjectCommandPreview    string
}

// --- Custom Message Types ---

// ErrorMsg signals an error occurred, potentially during an external command.
type ErrorMsg struct {
	Err error
}

// SuccessMsg signals a successful operation, potentially from an external command.
type SuccessMsg struct {
	Message string
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
