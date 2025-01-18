package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	tea "github.com/charmbracelet/bubbletea"
)

func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "ctrl+c", "esc":
		os.Exit(0)

	case "enter":
		filename := strings.TrimSpace(m.TempFilename)
		if filename == "" {
			return m, nil
		}

		// Build placeholders (this ensures "{example}" is in the map)
		placeholderMap := commands.BuildNamePlaceholders(filename)

		// Now run the command with that placeholder map
		if err := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholderMap); err != nil {
			fmt.Println("Command error:", err)
			return m, nil
		}

		m.CurrentScreen = app.ScreenMain
		return m, nil
	}

	if len(keyMsg.String()) == 1 {
		m.TempFilename += keyMsg.String()
	} else if keyMsg.String() == "backspace" && len(m.TempFilename) > 0 {
		m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
	}

	return m, nil
}

func ViewFilenamePrompt(m app.Model) string {
	return "\nEnter the new file/component name:\n\n" +
		"> " + m.TempFilename + "\n\n" +
		"(Press Enter to confirm | ESC/ctrl+c to quit)"
}
