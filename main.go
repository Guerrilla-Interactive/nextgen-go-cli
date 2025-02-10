package main

import (
	"fmt"
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// Add a new message type that will trigger quit after a delay.
type QuitAfterDelayMsg struct{}

// ProgramModel wraps app.Model so we can hold Update logic in one place.
type ProgramModel struct {
	M app.Model
}

// Init returns the Cmd that loads project info from screens.InitProjectCmd.
func (pm ProgramModel) Init() tea.Cmd {
	// This Cmd will eventually yield an Msg containing an updated app.Model (with ProjectPath set).
	return screens.InitProjectCmd(pm.M)
}

// Update handles incoming Msgs (both from Init commands and user interaction).
func (pm ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {

	// 1) If the message is an app.Model, it's likely from InitProjectCmd:
	case app.Model:
		pm.M = typedMsg
		return pm, nil

	// 2) Handle the asynchronous command finished message.
	case screens.CommandFinishedMsg:
		if typedMsg.Err != nil {
			// Optionally log or display the error.
			fmt.Println("Command finished with error:", typedMsg.Err)
		}
		// Update to installation details screen.
		pm.M.CurrentScreen = app.ScreenInstallDetails

	// 3) Handle window size message
	case tea.WindowSizeMsg:
		// Record terminal dimensions for layout purposes.
		pm.M.TerminalWidth = typedMsg.Width
		pm.M.TerminalHeight = typedMsg.Height
		return pm, nil

	case tea.KeyMsg:
		switch pm.M.CurrentScreen {
		case app.ScreenSelect:
			updatedM, cmd := screens.UpdateScreenSelect(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenMain:
			updatedM, cmd := screens.UpdateScreenMain(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenAll:
			updatedM, cmd := screens.UpdateScreenAll(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenFilenamePrompt:
			updatedM, cmd := screens.UpdateScreenFilenamePrompt(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenInstallDetails:
			updatedM, cmd := screens.UpdateInstallDetailsScreen(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		default:
			return pm, nil
		}
	}

	// For non-key messages or screens we didn't switch on, just return unchanged.
	return pm, nil
}

// View selects which screen's View function to call based on pm.M.CurrentScreen.
func (pm ProgramModel) View() string {
	switch pm.M.CurrentScreen {
	case app.ScreenSelect:
		return screens.ViewSelectScreen(pm.M)
	case app.ScreenMain:
		return screens.ViewMainScreen(pm.M)
	case app.ScreenAll:
		return screens.ViewAllScreen(pm.M)
	case app.ScreenFilenamePrompt:
		return screens.ViewFilenamePrompt(pm.M)
	case app.ScreenInstallDetails:
		return screens.ViewInstallDetailsScreen(pm.M)
	}
	return ""
}

// Here's an example of how to set the initial Model so that if
// the user was already "logged in" or had chosen "offline" previously,
// we skip directly to app.ScreenMain.
func main() {
	// Read from env or a file that stores whether the user was logged in/offline
	// in a previous session. This example just reads an environment variable:
	skipIntro := os.Getenv("SKIP_INTRO")

	// Build your initial model.
	// If skipIntro is "1" (or if you have stored isLoggedIn == true, etc.),
	// you'd set up your Model accordingly.
	initialModel := app.Model{
		IsLoggedIn: false, // or read from session
	}

	// Suppose setting SKIP_INTRO=1 means we skip the intro screen no matter what:
	if skipIntro == "1" {
		initialModel.IsLoggedIn = true
		initialModel.CurrentScreen = app.ScreenMain
	} else {
		// Otherwise, start on the "select" screen as usual.
		// (app.ScreenSelect is default, so you might leave it out.)
		initialModel.CurrentScreen = app.ScreenSelect
	}

	// Set default terminal dimensions so panels are anchored on first render.
	if initialModel.TerminalHeight == 0 {
		initialModel.TerminalHeight = 24
	}
	if initialModel.TerminalWidth == 0 {
		initialModel.TerminalWidth = 80
	}

	// Start the Bubble Tea program using ProgramModel as our root model.
	p := tea.NewProgram(
		ProgramModel{
			M: initialModel,
		},
		tea.WithAltScreen(),
	)
	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

// Main update function. Your program should call this.
func update(msg tea.Msg, m app.Model) (app.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// When a command finishes we either show the installation details with an error,
	// or simply show installation details and quit.
	case screens.CommandFinishedMsg:
		if msg.Err != nil {
			// Optionally log or update a field that holds error info.
			fmt.Println("Command finished with error:", msg.Err)
		}
		// Set the current screen to the installation details screen.
		m.CurrentScreen = app.ScreenInstallDetails
		// Option 1 - Immediately quit:
		// return m, tea.Quit

		// Option 2 - Wait for a key press on the install details screen to quit:
		return m, nil

	case tea.KeyMsg:
		// If we're on the installation details screen, any key press quits.
		if m.CurrentScreen == app.ScreenInstallDetails {
			return m, tea.Quit
		}
		// Delegate to screen-specific updates.
		switch m.CurrentScreen {
		case app.ScreenAll:
			return screens.UpdateScreenAll(m, msg)
		case app.ScreenFilenamePrompt:
			return screens.UpdateScreenFilenamePrompt(m, msg)
		case app.ScreenSelect:
			return screens.UpdateScreenSelect(m, msg)
		// ... add additional cases for other screens as needed.
		default:
			return m, nil
		}
	}

	return m, nil
}
