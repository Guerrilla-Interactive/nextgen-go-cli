package main

import (
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
		// Capture that updated model (which has ProjectPath set).
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
	}
	return ""
}

func main() {
	// Start the Bubble Tea program using ProgramModel as our root model.
	p := tea.NewProgram(
		ProgramModel{
			M: app.Model{}, // initial data
		},
	)
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
