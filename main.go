package main

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens"
	tea "github.com/charmbracelet/bubbletea"
)

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

	// 1) If the message is an app.Model, it’s likely from InitProjectCmd:
	case app.Model:
		pm.M = typedMsg
		return pm, nil

	// 2) If the message is a tea.KeyMsg, dispatch to the appropriate screen’s Update method:
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
		}
	}

	// For non-key messages or screens we didn’t switch on, just return unchanged.
	return pm, nil
}

// View selects which screen’s View function to call based on pm.M.CurrentScreen.
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
	}
	return ""
}

// Here’s an example of how to set the initial Model so that if
// the user was already "logged in" or had chosen "offline" previously,
// we skip directly to app.ScreenMain.
func main() {
	// Read from env or a file that stores whether the user was logged in/offline
	// in a previous session. This example just reads an environment variable:
	skipIntro := os.Getenv("SKIP_INTRO")

	// Build your initial model.
	// If skipIntro is "1" (or if you have stored isLoggedIn == true, etc.),
	// you’d set up your Model accordingly.
	initialModel := app.Model{
		IsLoggedIn: false, // or read from session
	}

	// Suppose setting SKIP_INTRO=1 means we skip the intro screen no matter what:
	if skipIntro == "1" {
		initialModel.IsLoggedIn = true
		initialModel.CurrentScreen = app.ScreenMain
	} else {
		// Otherwise, start on the “select” screen as usual.
		// (app.ScreenSelect is default, so you might leave it out.)
		initialModel.CurrentScreen = app.ScreenSelect
	}

	// Start the Bubble Tea program using ProgramModel as our root model.
	p := tea.NewProgram(
		ProgramModel{
			M: initialModel,
		},
	)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
