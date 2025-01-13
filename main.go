package main

import (
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens"
	tea "github.com/charmbracelet/bubbletea"
)

type ProgramModel struct {
	M app.Model
}

// Here we can return whatever initial command we want, e.g. to load project data:
func (pm ProgramModel) Init() tea.Cmd {
	return screens.InitProjectCmd(pm.M)
}

func (pm ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Bubble Tea calls Update for every message; if it's a KeyMsg, delegate to the screen’s update:
	switch typedMsg := msg.(type) {
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
	// For non-key messages or a default branch:
	return pm, nil
}

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
	// Start your Bubble Tea program:
	p := tea.NewProgram(ProgramModel{
		M: app.Model{},
	})
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
