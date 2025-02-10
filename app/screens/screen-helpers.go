package screens

import (
	"fmt"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// summarizeProjectStats returns a string with project stats.
func summarizeProjectStats(m app.Model) string {
	result := ""
	if len(m.RecognizedPkgs) == 0 {
		result += "    • None recognized packages\n"
	} else {
		// Render recognized packages in up to 6 columns using Lipgloss.
		result += renderPackagesHorizontally(m.RecognizedPkgs, 6)
	}
	return result
}

// renderPackagesHorizontally displays items in a grid of up to maxCols columns,
// without a fixed widt
func renderPackagesHorizontally(items []string, maxCols int) string {
	if len(items) == 0 {
		return ""
	}

	// Number of columns is either maxCols or fewer if we have fewer items.
	cols := maxCols
	if len(items) < cols {
		cols = len(items)
	}
	// Compute how many rows we need (integer ceiling).
	rows := (len(items) + cols - 1) / cols

	// No fixed width, just a small margin to the right for spacing.
	colStyle := lipgloss.NewStyle().
		MarginRight(2).
		Align(lipgloss.Left)

	var lines []string

	for r := 0; r < rows; r++ {
		var line string
		for c := 0; c < cols; c++ {
			index := c*rows + r
			if index >= len(items) {
				break
			}

			// Insert "•" before each item except the first in a row.
			if c > 0 {
				line += "•  "
			}

			item := items[index]
			line += colStyle.Render(item)
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	// Join all rows with a newline.
	return strings.Join(lines, "\n") + "\n"
}

// getItemName returns the label (and a bool if it's the last item).
func getItemName(m app.Model, index int) (string, bool) {
	// offset = len(commands.RecentUsed) + (len(commands.NextSteps) - 1)
	offset := len(commands.RecentUsed) + (len(commands.NextSteps) - 1)

	// If index == offset, we're on Logout/Login.
	if index == offset {
		if m.IsLoggedIn {
			return "Logout", true
		}
		return "Login", true
	}

	// If within recent commands:
	if index < len(commands.RecentUsed) {
		return commands.RecentUsed[index], false
	}

	// Otherwise, it's a NextStep.
	stepIndex := index - len(commands.RecentUsed)
	return commands.NextSteps[stepIndex], false
}

// recordCommand moves the chosen command to the front of RecentUsed, removing duplicates, limit to 8.
func recordCommand(m *app.Model, cmd string) {
	idx := -1
	for i, v := range commands.RecentUsed {
		if v == cmd {
			idx = i
			break
		}
	}
	if idx != -1 {
		// Remove it from old position
		commands.RecentUsed = append(commands.RecentUsed[:idx], commands.RecentUsed[idx+1:]...)
	}

	// Add to front
	commands.RecentUsed = append([]string{cmd}, commands.RecentUsed...)

	// Cap at 8
	if len(commands.RecentUsed) > 8 {
		commands.RecentUsed = commands.RecentUsed[:8]
	}

	m.TotalItems = len(commands.RecentUsed) + len(commands.NextSteps)
}

// renderItemsHorizontally is an example utility that can display a set of items in a row-based layout.
func renderItemsHorizontally(items []string, m *app.Model, offset, columns int) string {
	var outputLines []string
	var currentLine string

	for i, val := range items {
		if i != 0 && i%columns == 0 {
			outputLines = append(outputLines, currentLine)
			currentLine = ""
		}
		fullIndex := offset + i
		if m.SelectedIndex == fullIndex && m.CurrentScreen == app.ScreenMain {
			currentLine += app.HighlightStyle.Render("> "+val+" <") + "  "
		} else {
			currentLine += app.ChoiceStyle.Render(val) + "  "
		}
	}
	if currentLine != "" {
		outputLines = append(outputLines, currentLine)
	}

	return strings.Join(outputLines, "\n") + "\n"
}

// renderItemList is used for the NextSteps on the main screen.
func renderItemList(items []string, m app.Model, offset int) string {
	var out string
	for i, val := range items {
		fullIndex := offset + i
		if m.SelectedIndex == fullIndex && m.CurrentScreen == app.ScreenMain {
			out += app.HighlightStyle.Render("> "+val+" <") + "\n"
		} else {
			out += app.ChoiceStyle.Render(val) + "\n"
		}
	}
	return out
}

// requiresMultipleVars checks whether the command requires multiple variable inputs.
// It loads the command's JSON template and infers variable keys automatically.
func requiresMultipleVars(cmdName string) bool {
	spec := commands.GetCommandSpec(cmdName)
	if spec.TemplatePath == "" {
		return false
	}
	keys, err := commands.GetTemplateVariableKeys(spec)
	if err != nil {
		// On error we assume no extra variables are required.
		return false
	}
	// If more than one unique key is found, consider it a multi-variable command.
	return len(keys) > 1
}

// extractVariableKeys returns the list of variable keys inferred from the command's JSON template.
func extractVariableKeys(cmdName string) []string {
	spec := commands.GetCommandSpec(cmdName)
	if spec.TemplatePath == "" {
		return nil
	}
	keys, err := commands.GetTemplateVariableKeys(spec)
	if err != nil {
		return nil
	}
	return keys
}

// HandleCommandSelection centralizes what happens when a command is selected.
func HandleCommandSelection(m *app.Model, itemName string) *app.Model {
	// Always record the command so it appears at the top of RecentUsed:
	recordCommand(m, itemName)

	if itemName == commands.NextSteps[0] {
		m.CurrentScreen = app.ScreenAll
		m.AllCmdsIndex = 0
		m.AllCmdsTotal = len(commands.AllCommandNames()) + 1
		return m
	}

	// Check if the command requires multiple variable inputs by inferring keys from its JSON.
	if requiresMultipleVars(itemName) {
		m.PendingCommand = itemName
		m.MultipleVariables = true
		// Infer keys from the template (will now include any property variables, e.g. "Property: String")
		m.VariableKeys = extractVariableKeys(itemName)
		// Log the detected template variable keys.
		fmt.Printf("Detected template variable keys: %v\n", m.VariableKeys)
		m.CurrentVariableIndex = 0
		m.Variables = make(map[string]string)
		m.CurrentScreen = app.ScreenFilenamePrompt
		return m
	}

	// For "add " commands without multiple variables, use the single-variable prompt.
	if strings.HasPrefix(itemName, "add ") {
		m.PendingCommand = itemName
		m.CurrentScreen = app.ScreenFilenamePrompt
		return m
	}

	// Otherwise, run the command immediately.
	commands.RunCommand(itemName, m.ProjectPath, nil)
	// After running the command, show the installation details screen.
	m.CurrentScreen = app.ScreenInstallDetails
	return m
}

// -----------------------------------------------------------------------------
// New helper functions to create a unified container and to provide installation details
// -----------------------------------------------------------------------------

// baseContainer wraps the provided content in a nice Lipgloss border and padding.
func baseContainer(content string) string {
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		Padding(1, 2).
		Margin(1)
	return containerStyle.Render(content)
}

// ViewInstallDetailsScreen shows a nicely formatted installation details screen.
func ViewInstallDetailsScreen(m app.Model) string {
	installMsg := "Installation Complete!"
	details := fmt.Sprintf("Command executed: %s\nProject Path: %s\n", m.PendingCommand, m.ProjectPath)

	// Use m.CreatedFiles if available; otherwise fall back on commands.CreatedFiles.
	createdFiles := m.CreatedFiles
	if len(createdFiles) == 0 {
		createdFiles = commands.CreatedFiles
	}
	var fileLinks string
	if len(createdFiles) > 0 {
		fileLinks = "\nCreated Files:\n"
		for _, f := range createdFiles {
			// Format the file path as a stylized link.
			link := fmt.Sprintf("• %s", f)
			fileLinks += app.LinkStyle.Render(link) + "\n"
		}
	} else {
		fileLinks = app.HelpStyle.Render("No files were created.")
	}

	help := app.HelpStyle.Render("Press any key to exit.")
	content := app.TitleStyle.Render(installMsg) + "\n" +
		app.PathStyle.Render(m.ProjectPath) + "\n\n" +
		details + "\n" + fileLinks + "\n\n" + help
	return baseContainer(content)
}

// UpdateScreenInstallDetails handles input on the installation details screen.
// On any key press it quits the application.
func UpdateScreenInstallDetails(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	return m, tea.Quit
}

// CommandFinishedMsg is sent when an asynchronous command execution has completed.
type CommandFinishedMsg struct {
	Err error
}
