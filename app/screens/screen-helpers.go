package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxCommandHistory = 20 // Limit project-specific history size

// summarizeProjectStats returns a string with project stats.
func summarizeProjectStats(m app.Model) string {
	result := ""
	if len(m.RecognizedPkgs) == 0 {
		result += ""
	} else {
		// Group recognized packages for advanced display.
		groupedPkgs := groupRecognizedPackages(m.RecognizedPkgs)
		// Render grouped packages in up to 6 columns using Lipgloss.
		result += renderPackagesHorizontally(groupedPkgs, 6)
	}
	return result
}

// groupRecognizedPackages processes a list of package names, grouping React-based frameworks
// and CSS frameworks. For example:
//   - If "Next.js" (or Gatsby, etc.) is detected, only that candidate is kept (with a preference for Next.js).
//   - If multiple CSS frameworks are detected, they are summarized as "N CSS Packages".
func groupRecognizedPackages(pkgs []string) []string {
	// Define known react frameworks (all lower-case comparisons).
	reactFrameworks := map[string]bool{
		"next.js":      true,
		"gatsby":       true,
		"react-native": true,
		"remix":        true,
		"blitzjs":      true,
	}
	// Define known CSS frameworks (lower-case); add more as needed.
	cssFrameworks := map[string]bool{
		"tailwind css": true,
		"bootstrap":    true,
		"bulma":        true,
		"foundation":   true,
		"semantic-ui":  true,
		"material-ui":  true,
		"chakra ui":    true,
		"ant design":   true,
	}

	var finalPkgs []string
	var reactCandidate string
	cssCount := 0
	var cssCandidate string

	// For non-group packages, avoid duplicates.
	seen := map[string]bool{}

	for _, pkg := range pkgs {
		normalized := strings.ToLower(pkg)
		// If package is in the React frameworks group.
		if reactFrameworks[normalized] {
			// If no candidate selected yet, choose this one.
			if reactCandidate == "" {
				reactCandidate = pkg
			} else {
				// Give preference to "Next.js" if encountered.
				if normalized == "next.js" {
					reactCandidate = pkg
				}
			}
			continue
		}
		// For the base "react" itself, only consider it if no framework candidate was already found.
		if normalized == "react" {
			if reactCandidate == "" {
				reactCandidate = pkg
			}
			continue
		}
		// If package is in the CSS group.
		if cssFrameworks[normalized] {
			cssCount++
			if cssCandidate == "" {
				cssCandidate = pkg
			}
			continue
		}
		// For all other packages, add if not already added.
		if !seen[pkg] {
			finalPkgs = append(finalPkgs, pkg)
			seen[pkg] = true
		}
	}

	// Append the React candidate (if any) only once.
	if reactCandidate != "" {
		finalPkgs = append(finalPkgs, reactCandidate)
	}

	// Append CSS frameworks – if more than one CSS framework was detected, summarize the count.
	if cssCount > 0 {
		if cssCount == 1 {
			finalPkgs = append(finalPkgs, cssCandidate)
		} else {
			finalPkgs = append(finalPkgs, fmt.Sprintf("%d CSS Packages", cssCount))
		}
	}
	return finalPkgs
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

	// No fixed width, just a small margin to the right for spacing. Gray color.
	colStyle := lipgloss.NewStyle().
		MarginRight(2).
		Align(lipgloss.Left).
		Foreground(lipgloss.Color("#888"))

	var lines []string

	for r := 0; r < rows; r++ {
		var line string
		for c := 0; c < cols; c++ {
			index := c*rows + r
			if index >= len(items) {
				break
			}

			// Insert "•" before each item except the first in a row. Gray color.
			if c > 0 {
				line += lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("•  ")
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

// recordCommand moves the chosen command to the front of RecentUsed (session memory).
// Persistent history is now recorded after RunCommand.
func recordCommand(m *app.Model, cmd string) {
	// --- Reset HistorySaveStatus (no longer relevant here) ---
	m.HistorySaveStatus = ""

	// Only record commands that are not part of the action row or navigation commands.
	lower := strings.ToLower(cmd)
	excluded := map[string]bool{
		"undo":                     true,
		"redo":                     true,
		"show all my commands":     true,
		"view project stats":       true,
		"logoutorloginplaceholder": true,
		"paste from clipboard":     true,
	}
	if excluded[lower] {
		return
	}

	// --- Update In-Memory RecentUsed list (Keep this part) ---
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

	// --- Remove Persistent Project Command History Logic ---
	// (This logic is moved to be called after RunCommand)
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
	// Special handling for clipboard paste command
	if strings.ToLower(cmdName) == "paste from clipboard" {
		keys, err := commands.ExtractVariablesFromClipboard()
		if err != nil {
			return false
		}
		return len(keys) > 1
	}

	// Regular template handling (existing code)
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
	// Special handling for clipboard paste command
	if strings.ToLower(cmdName) == "paste from clipboard" {
		keys, err := commands.ExtractVariablesFromClipboard()
		if err != nil {
			return []string{"Filename"}
		}
		return keys
	}

	// Regular template handling (existing code)
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
// It now accepts the registry to record history.
func HandleCommandSelection(m *app.Model, registry *project.ProjectRegistry, itemName string) *app.Model {
	// Always record the command so it appears at the top of RecentUsed:
	recordCommand(m, itemName)

	if strings.ToLower(itemName) == "view project stats" {
		m.CurrentScreen = app.ScreenProjectStats
		return m
	}

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

	// For all "add" commands (case-insensitive), use the filename prompt.
	if strings.HasPrefix(strings.ToLower(itemName), "add ") {
		m.PendingCommand = itemName
		m.CurrentScreen = app.ScreenFilenamePrompt
		return m
	}

	// Otherwise, run the command immediately.
	// Pass registry to RunCommand
	commands.RunCommand(itemName, m.ProjectPath, nil, registry)
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
		Padding(1, 2).
		Margin(1)
	return containerStyle.Render(content)
}

func sideContainer(content string) string {
	containerStyle := lipgloss.NewStyle().
		Padding(1, 2)

	return containerStyle.Render(content)
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

// NEW: renderProjectInfoSection formats the common project info details.
func renderProjectInfoSection(m app.Model, registry *project.ProjectRegistry) string {
	var infoBuilder strings.Builder

	// --- Project Path --- (Keep this concise)
	if m.ProjectPath != "" {
		// Maybe shorten long paths?
		displayPath := m.ProjectPath
		// Example shortening (adjust logic as needed):
		// if len(displayPath) > 40 {
		// 	displayPath = "..." + displayPath[len(displayPath)-37:]
		// }
		infoBuilder.WriteString(app.PathStyle.Render(displayPath) + "\n")
	} else {
		infoBuilder.WriteString(app.ChoiceStyle.Render("Path N/A") + "\n")
	}
	infoBuilder.WriteString("\n") // Add a separator

	// --- Project Usage (Summary) ---
	infoBuilder.WriteString(app.SubtitleStyle.Render("Usage:") + "\n")
	if registry != nil && m.ProjectPath != "" {
		if projectInfo, found := registry.GetProject(m.ProjectPath); found {
			infoBuilder.WriteString(fmt.Sprintf("  Count: %d\n", projectInfo.UsageCount))
			lastAccess := time.Unix(projectInfo.LastAccessTime, 0)
			infoBuilder.WriteString(fmt.Sprintf("  Last: %s\n", lastAccess.Format("Jan 2, 3:04 PM")))
		} else {
			infoBuilder.WriteString(app.ChoiceStyle.Render("  (Not recorded yet)\n"))
		}
	} else {
		infoBuilder.WriteString(app.ChoiceStyle.Render("  (Registry N/A)\n"))
	}

	// No extra padding here, let the caller handle container padding
	return infoBuilder.String()
}
