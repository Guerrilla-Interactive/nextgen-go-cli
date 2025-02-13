package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenFilenamePrompt handles input for both single and multiple variables.
func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg) (app.Model, tea.Cmd) {
	// Check for arrow keys (actual arrow keys) to change focus.
	switch keyMsg.String() {
	case "up":
		// Pressing the up arrow focuses the "[Back]" button.
		m.PromptOptionFocused = true
		return m, nil
	case "down":
		// Pressing the down arrow returns focus to the input field.
		m.PromptOptionFocused = false
		return m, nil
	}

	// If the "[Back]" button is focused, process only the Enter key.
	if m.PromptOptionFocused {
		if keyMsg.String() == "enter" {
			m.CurrentScreen = app.ScreenMain
			m.TempFilename = ""
			m.LivePreview = ""
			m.PromptOptionFocused = false
		}
		return m, nil
	}

	// Check if we are in multi-variable mode.
	if m.MultipleVariables {
		switch keyMsg.String() {
		case "ctrl+c":
			os.Exit(0)
		case "esc", "b", "B":
			// Go back to recent commands.
			m.CurrentScreen = app.ScreenMain
			m.TempFilename = ""
			m.LivePreview = ""
			// Also ensure we are not in prompt option focus.
			m.PromptOptionFocused = false
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.TempFilename)
			if value == "" {
				return m, nil
			}
			// Store the given input for the current variable.
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

				// Update the live preview.
				if preview, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholders, m.ProjectPath); err == nil {
					m.LivePreview = preview
				} else {
					m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
				}

				// Update the current screen to avoid later index-out-of-range in the view.
				m.CurrentScreen = app.ScreenInstallDetails

				// Run the command with the built placeholders.
				return m, func() tea.Msg {
					err := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholders)
					return CommandFinishedMsg{Err: err}
				}
			}
			// Update live preview for multi-variable mode.
			{
				tempVars := make(map[string]string)
				for k, v := range m.Variables {
					tempVars[k] = v
				}
				if m.CurrentVariableIndex < len(m.VariableKeys) {
					currentKey := m.VariableKeys[m.CurrentVariableIndex]
					if strings.TrimSpace(m.TempFilename) == "" {
						tempVars[currentKey] = "Filename"
					} else {
						tempVars[currentKey] = m.TempFilename
					}
				}
				placeholders := commands.BuildPlaceholders(tempVars)
				if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
					if preview, err := commands.GeneratePreviewFileTreeFromClipboard(placeholders, m.ProjectPath); err == nil {
						m.LivePreview = preview
					} else {
						m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
					}
				} else {
					if preview, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholders, m.ProjectPath); err == nil {
						m.LivePreview = preview
					} else {
						m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
					}
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
	case "ctrl+c":
		os.Exit(0)
	case "esc", "b", "B":
		// Go back to recent commands.
		m.CurrentScreen = app.ScreenMain
		m.TempFilename = ""
		m.LivePreview = ""
		return m, nil
	case "enter":
		filename := strings.TrimSpace(m.TempFilename)
		if filename == "" {
			return m, nil
		}

		// Determine the placeholder map.
		spec := commands.GetCommandSpec(m.PendingCommand)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: filename})
		} else {
			if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Filename": filename})
			} else {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": filename})
			}
		}

		// Update live preview using the appropriate helper.
		if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
			if preview, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath); err == nil {
				m.LivePreview = preview
			} else {
				m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		} else {
			if preview, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath); err == nil {
				m.LivePreview = preview
			} else {
				m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		}

		// Run the command with the built placeholders.
		return m, func() tea.Msg {
			err := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholderMap)
			return CommandFinishedMsg{Err: err}
		}
	}
	// If the key is a single character (and not one of our reserved navigation keys),
	// then append it to the input. This lets you use letters (or digits, etc.) for the input.
	if len(keyMsg.String()) == 1 {
		m.TempFilename += keyMsg.String()
	} else if keyMsg.String() == "backspace" && len(m.TempFilename) > 0 {
		m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
	}

	// In single variable mode, update live preview.
	{
		input := m.TempFilename
		if strings.TrimSpace(input) == "" {
			input = "Filename"
		}
		// Use the template variable key if available.
		spec := commands.GetCommandSpec(m.PendingCommand)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: input})
		} else {
			if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Filename": input})
			} else {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": input})
			}
		}
		if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
			if preview, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath); err == nil {
				m.LivePreview = preview
			} else {
				m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		} else {
			if preview, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath); err == nil {
				m.LivePreview = preview
			} else {
				m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		}
	}
	return m, nil
}

// ViewFilenamePrompt displays the proper prompt based on the current mode.
func ViewFilenamePrompt(m app.Model) string {
	// Define a cursor style and determine whether to show the input cursor
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	inputCursor := ""
	// blink the cursor if the input field is focused

	if !m.PromptOptionFocused {
		inputCursor = cursorStyle.Render("▎")
		cursor.Blink()

	}

	var prompt string
	if m.MultipleVariables {
		if m.CurrentVariableIndex >= len(m.VariableKeys) {
			prompt = "\nProcessing command... please wait.\n"
		} else {
			currentKey := m.VariableKeys[m.CurrentVariableIndex]
			prompt = fmt.Sprintf("Enter value for %s:\n> %s%s", currentKey, m.TempFilename, inputCursor)
		}
	} else {
		prompt = fmt.Sprintf("Enter the new file/component name:\n> %s%s", m.TempFilename, inputCursor)
	}

	// Build the input panel with a border that changes based on focus.
	var inputBorderStyle lipgloss.Style
	if m.PromptOptionFocused {
		// [Back] is focused so render the input field "blurred" with a gray border.
		inputBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)
	} else {
		// When the input field is focused, use a white border with extra padding.
		inputBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("15")).
			Padding(1, 2)
	}
	inputPanel := inputBorderStyle.Render(prompt)

	// Build the "[Back]" button row. It is highlighted when focused.
	var backButton string
	if m.PromptOptionFocused {
		backButton = app.HighlightStyle.Render("[Back]")
	} else {
		backButton = app.HelpStyle.Render("[Back]")
	}

	// Place the "[Back]" button above the input panel.
	leftPanel := lipgloss.JoinVertical(lipgloss.Left, backButton, inputPanel)

	// If LivePreview is empty, compute a default preview using default placeholder values.
	preview := m.LivePreview
	if strings.TrimSpace(preview) == "" {
		// Default input (used when no input is provided)
		input := "Filename"
		// Retrieve the command spec and variable keys.
		spec := commands.GetCommandSpec(m.PendingCommand)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: input})
		} else {
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": input})
		}
		if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
			if pv, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath); err == nil {
				preview = pv
			} else {
				preview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		} else {
			if pv, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath); err == nil {
				preview = pv
			} else {
				preview = fmt.Sprintf("Preview unavailable: %v", err)
			}
		}
	}

	// Prepend header with package icon and current folder name.
	folderName := filepath.Base(m.ProjectPath)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("�� %s", folderName))
	preview = header + "\n\n" + preview

	// Build the right panel (the file tree preview) using the updated preview.
	rightPanel := sideContainer(preview)

	// Join the anchored left panel and the right panel horizontally with bottom alignment,
	// then append the help notice.
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Bottom, leftPanel, rightPanel),
		app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)"),
	)
}
