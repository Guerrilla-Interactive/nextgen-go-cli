package shared

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxCommandHistory = 20 // Keep internal constant

// SummarizeProjectStats - Exported for use in screens.
func SummarizeProjectStats(m app.Model) string {
	result := ""
	if len(m.RecognizedPkgs) == 0 {
		result += ""
	} else {
		// Group recognized packages for advanced display.
		groupedPkgs := groupRecognizedPackages(m.RecognizedPkgs)
		// Render grouped packages in up to 6 columns using Lipgloss.
		result += RenderPackagesHorizontally(groupedPkgs, 6)
	}
	return result
}

// GroupRecognizedPackages - Keep internal for now.
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

// RenderPackagesHorizontally - Exported as it's called by SummarizeProjectStats.
func RenderPackagesHorizontally(items []string, maxCols int) string {
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

// RecordCommand updates in-memory RecentUsed list. Exported.
func RecordCommand(m *app.Model, cmd string) {
	// --- Reset HistorySaveStatus (no longer relevant here) ---
	m.HistorySaveStatus = ""

	// Define excluded commands locally
	excluded := map[string]bool{
		"undo":                     true,
		"redo":                     true,
		"show all my commands":     true,
		"view settings":            true,
		"logoutorloginplaceholder": true,
		"paste from clipboard":     true,
	}

	// Only record commands that are not part of the action row or navigation commands.
	lower := strings.ToLower(cmd)
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

	// --- Remove Persistent Project Command History Logic ---
	// (This logic is moved to be called after RunCommand)
}

// RenderItemList renders a list for UI. Exported.
func RenderItemList(items []string, m app.Model, offset int) string {
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

// RequiresMultipleVars checks if command needs multiple inputs. Exported.
func RequiresMultipleVars(cmdName, projectPath string, registry *project.ProjectRegistry) bool {
	// Special handling for clipboard paste command
	if strings.ToLower(cmdName) == "paste from clipboard" {
		keys, err := commands.ExtractVariablesFromClipboard()
		if err != nil {
			return false
		}
		return len(keys) > 1
	}

	// Use new function to get keys
	keys, err := commands.GetCommandVariableKeys(cmdName, projectPath, registry)
	if err != nil {
		// On error we assume no extra variables are required.
		return false
	}
	// If more than one unique key is found, consider it a multi-variable command.
	return len(keys) > 1
}

// ExtractVariableKeys gets variable keys for a command. Exported.
func ExtractVariableKeys(cmdName, projectPath string, registry *project.ProjectRegistry) []string {
	// Special handling for clipboard paste command
	if strings.ToLower(cmdName) == "paste from clipboard" {
		keys, err := commands.ExtractVariablesFromClipboard()
		if err != nil {
			return []string{"Filename"}
		}
		return keys
	}

	// Use new function to get keys
	keys, err := commands.GetCommandVariableKeys(cmdName, projectPath, registry)
	if err != nil {
		return nil // Return nil on error
	}
	return keys
}

// HandleCommandSelection centralizes command selection logic. Exported.
func HandleCommandSelection(m *app.Model, registry *project.ProjectRegistry, itemName string) (*app.Model, tea.Cmd) {
	// Always record the command so it appears at the top of RecentUsed:
	RecordCommand(m, itemName)

	if strings.ToLower(itemName) == "view settings" {
		m.CurrentScreen = app.ScreenSettings
		return m, nil
	}

	if itemName == commands.NextSteps[0] {
		m.CurrentScreen = app.ScreenAll
		m.AllCmdsIndex = 0
		m.AllCmdsTotal = len(commands.AllCommandNames()) + 1
		return m, nil
	}

	// Detect composite commands (args/run) and route to a choice prompt
	if data, _, err := commands.LoadTemplateBytesForName(itemName, m.ProjectPath, registry); err == nil {
		if commands.IsCompositeTemplate(data) {
			slugs, _ := commands.GetCompositeRunSlugs(data)
			if len(slugs) > 0 {
				m.ChoiceOptionNames = []string{}
				m.ChoiceTargetSlugs = []string{}
				for _, s := range slugs {
					m.ChoiceOptionNames = append(m.ChoiceOptionNames, commands.ResolveCommandTitleBySlug(s))
					m.ChoiceTargetSlugs = append(m.ChoiceTargetSlugs, s)
				}
				m.ChoiceIndex = 0
				// Prefill preview for the first choice so it's visible immediately
				if len(m.ChoiceTargetSlugs) > 0 {
					first := m.ChoiceTargetSlugs[0]
					if keys, err := commands.GetCommandVariableKeys(first, m.ProjectPath, registry); err == nil {
						var ph map[string]string
						if len(keys) > 0 {
							ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
						} else {
							ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
						}
						if pv, perr := commands.GeneratePreviewFileTree(first, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
							m.FileTreePreview = pv
							m.CurrentPreviewType = "file-tree"
						}
					}
				}
				m.CurrentScreen = app.ScreenChoicePrompt
				return m, nil
			}
		}
		// Check autoBrowseRoot
		var t struct {
			AutoBrowseRoot string `json:"autoBrowseRoot"`
		}
		if json.Unmarshal(data, &t) == nil && strings.TrimSpace(t.AutoBrowseRoot) != "" {
			root := strings.TrimSpace(t.AutoBrowseRoot)
			if children, e := commands.ListNativeChildren(root); e == nil && len(children) > 0 {
				m.ChoiceOptionNames = []string{}
				m.ChoiceTargetSlugs = []string{}
				for _, c := range children {
					if c.IsDir {
						m.ChoiceOptionNames = append(m.ChoiceOptionNames, c.Name+"/")
						m.ChoiceTargetSlugs = append(m.ChoiceTargetSlugs, c.Path+"/")
					} else {
						name := strings.TrimSuffix(c.Name, filepath.Ext(c.Name))
						m.ChoiceOptionNames = append(m.ChoiceOptionNames, name)
						m.ChoiceTargetSlugs = append(m.ChoiceTargetSlugs, c.Path)
					}
				}
				m.ChoiceIndex = 0
				// Prefill preview for the first target so it's visible immediately
				if len(m.ChoiceTargetSlugs) > 0 {
					first := m.ChoiceTargetSlugs[0]
					if strings.HasPrefix(first, "native-commands/") && strings.HasSuffix(first, "/") {
						if nearest, ok := commands.FindFirstJSONUnder(strings.TrimSuffix(first, "/")); ok {
							keys, _ := commands.GetCommandVariableKeys(nearest, m.ProjectPath, registry)
							var ph map[string]string
							if len(keys) > 0 {
								ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
							} else {
								ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
							}
							if b, rerr := commands.ReadEmbeddedTemplate(nearest); rerr == nil {
								if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
									m.FileTreePreview = pv
									m.CurrentPreviewType = "file-tree"
								}
							}
						}
					} else if strings.HasPrefix(first, "native-commands/") && strings.HasSuffix(first, ".json") {
						keys, _ := commands.GetCommandVariableKeys(first, m.ProjectPath, registry)
						var ph map[string]string
						if len(keys) > 0 {
							ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
						} else {
							ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
						}
						if b, rerr := commands.ReadEmbeddedTemplate(first); rerr == nil {
							if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
								m.FileTreePreview = pv
								m.CurrentPreviewType = "file-tree"
							}
						}
					} else {
						keys, _ := commands.GetCommandVariableKeys(first, m.ProjectPath, registry)
						var ph map[string]string
						if len(keys) > 0 {
							ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
						} else {
							ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
						}
						if pv, perr := commands.GeneratePreviewFileTree(first, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
							m.FileTreePreview = pv
							m.CurrentPreviewType = "file-tree"
						}
					}
				}
				m.CurrentScreen = app.ScreenChoicePrompt
				return m, nil
			}
		}
	}

	// Check if the command requires variables using the updated function
	if RequiresMultipleVars(itemName, m.ProjectPath, registry) {
		m.PendingCommand = itemName
		m.MultipleVariables = true
		// Get keys using the updated function
		m.VariableKeys = ExtractVariableKeys(itemName, m.ProjectPath, registry)
		fmt.Printf("Detected template variable keys: %v\n", m.VariableKeys)
		m.CurrentVariableIndex = 0
		m.Variables = make(map[string]string)
		m.CurrentScreen = app.ScreenFilenamePrompt
		// We don't call UpdateFilenamePromptPreview here; the prompt screen handles its own preview.
		return m, cursor.Blink // Return blink command for the prompt
	}

	// Check for commands that require only a single variable (like most "add" commands)
	keys, err := commands.GetCommandVariableKeys(itemName, m.ProjectPath, registry)
	if err != nil {
		m.HistorySaveStatus = fmt.Sprintf("Error checking command '%s': %v", itemName, err)
		return m, nil
	}
	if len(keys) > 0 { // If *any* keys found, go to prompt (multi-var handled above)
		m.PendingCommand = itemName
		m.MultipleVariables = false // Explicitly set to false for single var mode
		m.VariableKeys = keys       // Store the keys even for single var mode
		m.CurrentScreen = app.ScreenFilenamePrompt
		// We don't call UpdateFilenamePromptPreview here; the prompt screen handles its own preview.
		return m, cursor.Blink // Return blink command for the prompt
	}

	// Otherwise (no keys required), run the command immediately.
	m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", itemName)
	// Ensure registry is passed to RunCommand
	runCmd := commands.RunCommand(itemName, m.ProjectPath, nil, registry)
	// After starting the command, show the installation details screen (or a loading screen).
	m.CurrentScreen = app.ScreenInstallDetails
	return m, runCmd // Return the command to execute
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

// BaseContainer wraps content in a border. Exported.
func BaseContainer(content string) string {
	containerStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Margin(1)
	return containerStyle.Render(content)
}

// SideContainer wraps content without border. Exported.
func SideContainer(content string) string {
	containerStyle := lipgloss.NewStyle().
		Padding(1, 2)

	return containerStyle.Render(content)
}

// UpdateScreenInstallDetails handles input for install screen. Exported.
// This function was previously commented out, now uncommented and Exported.
func UpdateScreenInstallDetails(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	return m, tea.Quit
}

// RenderProjectInfoSection formats project info. Exported.
func RenderProjectInfoSection(m app.Model, registry *project.ProjectRegistry) string {
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

// UpdateFilenamePromptPreview // MOVED to prompt package
/*
func UpdateFilenamePromptPreview(m app.Model, registry *project.ProjectRegistry) app.Model {
    ...
}
*/
