package prompt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenFilenamePrompt handles input for both single and multiple variables.
// It now accepts the registry to pass down to RunCommand.
func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
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
			m.FileTreePreview = ""
			m.StatsPreview = ""
			m.CurrentPreviewType = "none"
			m.PromptOptionFocused = false
		}
		return m, nil
	}

	// Check if we are in multi-variable mode.
	if m.MultipleVariables {
		switch keyMsg.String() {
		case "ctrl+c":
			os.Exit(0)
		case "esc":
			// Go back to recent commands.
			m.CurrentScreen = app.ScreenMain
			m.TempFilename = ""
			m.FileTreePreview = ""
			m.StatsPreview = ""
			m.CurrentPreviewType = "none"
			// Also ensure we are not in prompt option focus.
			m.PromptOptionFocused = false
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.TempFilename)
			if value == "" {
				return m, nil
			}
			currentKey := m.VariableKeys[m.CurrentVariableIndex]
			m.Variables[currentKey] = value
			m.TempFilename = ""
			m.CurrentVariableIndex++

			if m.CurrentVariableIndex >= len(m.VariableKeys) {
				// Record only the base command (not inputs) to recent
				sharedScreens.RecordCommand(&m, m.PendingCommand)
				mainValue := m.Variables[m.VariableKeys[0]]
				extraVars := make(map[string]string)
				for i := 0; i < len(m.VariableKeys); i++ {
					key := m.VariableKeys[i]
					extraVars[key] = m.Variables[key]
				}
				placeholders := commands.BuildMultiPlaceholders(mainValue, extraVars)
				// --- DEBUG ---
				fmt.Printf("DEBUG [Multi-Var Enter]: Placeholders: %+v\n", placeholders)
				// -------------
				m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", m.PendingCommand)
				m.CurrentScreen = app.ScreenInstallDetails
				runCmd := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholders, registry)
				return m, runCmd
			} else {
				// Still more variables to collect, update preview for next prompt
				m = updateFilenamePromptPreview(m, registry)
				return m, cursor.Blink // Return blink for the next input field
			}
		case "backspace":
			if len(m.TempFilename) > 0 {
				m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
			}
			// Update preview immediately after backspace
			m = updateFilenamePromptPreview(m, registry)
			return m, cursor.Blink
		default:
			// Append single character inputs.
			if len(keyMsg.String()) == 1 {
				m.TempFilename += keyMsg.String()
			}
			// Update preview immediately after character input
			m = updateFilenamePromptPreview(m, registry)
			return m, cursor.Blink
		}
	}
	// Single variable mode.
	switch keyMsg.String() {
	case "ctrl+c":
		os.Exit(0)
	case "esc":
		// Go back to recent commands.
		m.CurrentScreen = app.ScreenMain
		m.TempFilename = ""
		m.FileTreePreview = ""
		m.StatsPreview = ""
		m.CurrentPreviewType = "none"
		return m, nil
	case "enter":
		filename := strings.TrimSpace(m.TempFilename)
		if filename == "" {
			return m, nil
		}

		// Determine the placeholder map.
		keys, err := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
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
		// --- DEBUG ---
		fmt.Printf("DEBUG [Single-Var Enter]: Placeholders: %+v\n", placeholderMap)
		// -------------

		// --- Handle Clipboard Saving Separately (Before Running Command) ---
		if strings.ToLower(m.PendingCommand) == "paste from clipboard" && filename != "" {
			if registry != nil {
				commandNameToSave := filename // Default
				clipboardContent, readErr := clipboard.ReadAll()
				if readErr == nil {
					type cmdMeta struct {
						Title string `json:"title"`
						Type  string `json:"_type"`
					}
					var meta cmdMeta
					if json.Unmarshal([]byte(clipboardContent), &meta) == nil && meta.Type == "command" && meta.Title != "" {
						commandNameToSave = meta.Title
					}
				}

				clipboardContentToSave := clipboardContent // Use content read above if possible
				if clipboardContentToSave == "" && readErr != nil {
					clipboardContentToSave, _ = clipboard.ReadAll() // Try reading again
				}
				if clipboardContentToSave != "" {
					if err := commands.UpsertClipboardCommand(registry, commandNameToSave, clipboardContentToSave); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Warning: Failed to save clipboard command: %v", err)
					} else {
						m.HistorySaveStatus = fmt.Sprintf("Saved clipboard as command: %s", commandNameToSave)
					}
				} else {
					// Failed to read clipboard for saving
					m.HistorySaveStatus = "Warning: Could not read clipboard to save command."
				}
			}
		}
		// --- End Clipboard Saving ---

		// Record only the base command (not inputs) to recent
		sharedScreens.RecordCommand(&m, m.PendingCommand)
		// Set status and screen before returning command
		if m.HistorySaveStatus == "" { // Don't overwrite clipboard save status unless empty
			m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", m.PendingCommand)
		}
		m.CurrentScreen = app.ScreenInstallDetails

		// Get the command to run
		runCmd := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholderMap, registry)
		return m, runCmd
	}
	// If the key is a single character (and not one of our reserved navigation keys),
	// then append it to the input. This lets you use letters (or digits, etc.) for the input.
	if len(keyMsg.String()) == 1 {
		m.TempFilename += keyMsg.String()
	} else if keyMsg.String() == "backspace" && len(m.TempFilename) > 0 {
		m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
	}

	// Regenerate preview after input change in single-var mode
	m = updateFilenamePromptPreview(m, registry)

	return m, cursor.Blink // Return blink for single-var mode input
}

// ViewFilenamePrompt displays the proper prompt based on the current mode.
func ViewFilenamePrompt(m app.Model, registry *project.ProjectRegistry) string {
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
	leftPanelWidth := 50 // Example fixed width, adjust as needed
	leftPanel := lipgloss.NewStyle().
		Width(leftPanelWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, backButton, inputPanel))

	// If LivePreview is empty, compute a default preview using default placeholder values.
	preview := m.FileTreePreview
	if strings.TrimSpace(preview) == "" {
		// Default input (used when no input is provided)
		input := "Filename"
		// Retrieve the command spec and variable keys.
		keys, err := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
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
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	preview = header + "\n\n" + preview

	// Build the right panel style WITHOUT explicit width
	rightPanelStyle := lipgloss.NewStyle().
		Padding(1, 2) // Apply padding as needed
	// REMOVED explicit Width
	rightPanel := rightPanelStyle.Render(preview)

	// Join the panels horizontally
	// Lipgloss should handle distributing the width
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Bottom, leftPanel, "  ", rightPanel) // Add space

	finalView := lipgloss.JoinVertical(lipgloss.Left,
		combinedPanes, // Use the combined panel layout
		app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)"),
	)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}

// updateFilenamePromptPreview generates the file tree preview for the filename prompt screen.
// Moved here from shared/screen-helpers.go and made internal.
func updateFilenamePromptPreview(m app.Model, registry *project.ProjectRegistry) app.Model {
	var placeholderMap map[string]string

	// Determine the correct keys first using the new function
	keys, err := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
	if err != nil {
		// Handle error getting keys, maybe set preview to error message?
		m.FileTreePreview = fmt.Sprintf("Error getting keys for preview: %v", err)
		m.CurrentPreviewType = "none"
		return m
	}

	if m.MultipleVariables {
		// Build placeholders from the current state of variables
		placeholders := make(map[string]string)
		// Use the 'keys' obtained from GetCommandVariableKeys
		for i, key := range keys {
			if val, ok := m.Variables[key]; ok && val != "" {
				placeholders[key] = val
			} else if i == m.CurrentVariableIndex && m.TempFilename != "" {
				placeholders[key] = m.TempFilename // Use current input for the active field
			} else {
				placeholders[key] = "<" + key + ">" // Default placeholder
			}
		}
		placeholderMap = commands.BuildPlaceholders(placeholders)
	} else {
		// Single variable mode
		variableName := "Value" // Default if no keys found
		// Use the 'keys' obtained from GetCommandVariableKeys
		if len(keys) > 0 {
			variableName = keys[0]
		}
		placeholders := map[string]string{variableName: m.TempFilename}
		if m.TempFilename == "" {
			placeholders[variableName] = "<" + variableName + ">"
		}
		placeholderMap = commands.BuildPlaceholders(placeholders)
	}

	// --- Generate preview using the correct function ---
	var pv string
	var previewErr error
	if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
		pv, previewErr = commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath)
	} else {
		pv, previewErr = commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath)
	}
	// --- End preview generation ---

	if previewErr == nil && strings.TrimSpace(pv) != "" {
		m.FileTreePreview = pv
		m.CurrentPreviewType = "file-tree"
	} else {
		m.FileTreePreview = ""
		m.CurrentPreviewType = "none"
	}

	return m
}
