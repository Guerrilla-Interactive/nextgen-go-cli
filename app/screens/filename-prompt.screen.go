package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	tea "github.com/charmbracelet/bubbletea"
)

// UpdateScreenFilenamePrompt handles input for both single and multiple variables.
func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg) (app.Model, tea.Cmd) {
	// Check if we are in multi-variable mode.
	if m.MultipleVariables {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
			os.Exit(0)
		case "enter":
			value := strings.TrimSpace(m.TempFilename)
			if value == "" {
				return m, nil
			}
			// Get the current variable key and store the value.
			currentKey := m.VariableKeys[m.CurrentVariableIndex]
			m.Variables[currentKey] = value

			// Reset TempFilename for the next input.
			m.TempFilename = ""
			m.CurrentVariableIndex++

			// Check if all variables have been collected.
			if m.CurrentVariableIndex >= len(m.VariableKeys) {
				// Use the first variable as "Main" and the rest as extra variables.
				mainValue := m.Variables[m.VariableKeys[0]]
				extraVars := make(map[string]string)
				for i := 1; i < len(m.VariableKeys); i++ {
					extraVars[m.VariableKeys[i]] = m.Variables[m.VariableKeys[i]]
				}
				placeholders := commands.BuildMultiPlaceholders(mainValue, extraVars)

				// Run the command with the built placeholders.
				return m, func() tea.Msg {
					err := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholders)
					return CommandFinishedMsg{Err: err}
				}
			}
			return m, nil
		case "backspace":
			if len(m.TempFilename) > 0 {
				m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
			}
		default:
			// Append single character inputs.
			if len(keyMsg.String()) == 1 {
				m.TempFilename += keyMsg.String()
			}
		}
		return m, nil
	}

	// Single variable mode.
	switch keyMsg.String() {
	case "ctrl+c", "esc":
		os.Exit(0)
	case "enter":
		filename := strings.TrimSpace(m.TempFilename)
		if filename == "" {
			return m, nil
		}

		// Instead of always using "Main", check if the pending command's template
		// implies a different variable key.
		spec := commands.GetCommandSpec(m.PendingCommand)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		// If exactly one key is found (e.g. "ComponentName"), then use that
		if err == nil && len(keys) == 1 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: filename})
		} else {
			// Otherwise fallback on the older behavior using "Main"
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": filename})
		}

		// Run the command with that placeholder map.
		return m, func() tea.Msg {
			err := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholderMap)
			return CommandFinishedMsg{Err: err}
		}
	}

	if len(keyMsg.String()) == 1 {
		m.TempFilename += keyMsg.String()
	} else if keyMsg.String() == "backspace" && len(m.TempFilename) > 0 {
		m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
	}

	return m, nil
}

// ViewFilenamePrompt displays the proper prompt based on the current mode.
func ViewFilenamePrompt(m app.Model) string {
	var prompt string
	if m.MultipleVariables {
		// Prompt for the current variable whose value is being collected.
		currentKey := m.VariableKeys[m.CurrentVariableIndex]
		prompt = fmt.Sprintf("\nEnter value for %s:\n\n> %s\n\n(Press Enter to confirm | ESC/ctrl+c to quit)", currentKey, m.TempFilename)
	} else {
		prompt = "\nEnter the new file/component name:\n\n" +
			"> " + m.TempFilename + "\n\n" +
			"(Press Enter to confirm | ESC/ctrl+c to quit)"
	}
	// Wrap output in a styled container.
	return baseContainer(prompt)
}
