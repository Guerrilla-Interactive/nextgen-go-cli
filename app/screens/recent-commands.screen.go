package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actionRow defines the dedicated Action Row commands.
var actionRow = []string{"undo", "redo", "paste from clipboard", "view project stats"}

// filterRecentUsed returns recent commands filtered to remove any that are in the action row.
func filterRecentUsed(items []string) []string {
	var out []string
	for _, item := range items {
		lower := strings.ToLower(item)
		if lower == "undo" || lower == "redo" || lower == "paste from clipboard" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// totalNavCount returns the total number of navigable items across the Recent and Action groups.
func totalNavCount() int {
	return len(filterRecentUsed(commands.RecentUsed)) + len(actionRow)
}

// getItemName returns the command name for a given navigation index.
func getItemName(m app.Model, index int) (string, bool) {
	actionCount := len(actionRow)
	recent := filterRecentUsed(commands.RecentUsed)
	recentCount := len(recent)

	if index < actionCount {
		return actionRow[index], false
	} else if index < actionCount+recentCount {
		return recent[index-actionCount], false
	}
	return "", false
}

// UpdateScreenMain handles input for the main screen with "smart" arrow navigation:
//   - 3 columns × 5 rows in column-major order for RecentUsed.
//   - Pressing ↓ on the bottom row goes to the first NextSteps item ("Show all my commands");
//     pressing ↓ again goes to "Back"; pressing ↓ again wraps to the top of RecentUsed.
//   - Pressing ↑ on the top row goes to the last NextSteps item ("Back") if it exists; pressing ↑ again
//     moves to the first NextSteps, pressing ↑ again returns to the bottom of RecentUsed.
//   - SPECIAL REQUEST: When ↑ from the first NextSteps item ("Show all my commands"),
//     select the bottom of the first column in RecentUsed (index=4 if we have ≥5 commands).
func UpdateScreenMain(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	// First, compute group counts for navigation.
	recentFiltered := filterRecentUsed(commands.RecentUsed)
	actionCount := len(actionRow)
	recentCount := len(recentFiltered)

	// With the new ordering, the Action Row comes first.
	var group string
	if m.SelectedIndex < actionCount {
		group = "action"
	} else {
		group = "recent"
	}

	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "left", "h":
		// Only operate left/right if in the Action Row.
		if group == "action" {
			currentActionIndex := m.SelectedIndex // already 0-based in action group
			if currentActionIndex == 0 {
				currentActionIndex = actionCount - 1
			} else {
				currentActionIndex--
			}
			m.SelectedIndex = currentActionIndex
			m.LastActionIndex = currentActionIndex
			// Update preview based on new selection
			m = updatePreview(m)
		}

	case "right", "l":
		if group == "action" {
			currentActionIndex := m.SelectedIndex
			if currentActionIndex == actionCount-1 {
				currentActionIndex = 0
			} else {
				currentActionIndex++
			}
			m.SelectedIndex = currentActionIndex
			m.LastActionIndex = currentActionIndex
			// Update preview based on new selection
			m = updatePreview(m)
		}

	case "up", "k":
		if group == "action" {
			// If in the Action Row, press up to jump to the bottom of the Recently Used group.
			if recentCount > 0 {
				m.SelectedIndex = actionCount + recentCount - 1
			} else {
				m.SelectedIndex = 0
			}
		} else { // group == "recent"
			// When at the very top of the Recently Used group (index == actionCount),
			// jump to the previously highlighted Action Row item.
			if m.SelectedIndex == actionCount {
				if m.LastActionIndex >= 0 && m.LastActionIndex < actionCount {
					m.SelectedIndex = m.LastActionIndex
				} else {
					m.SelectedIndex = 0
				}
			} else {
				// Otherwise, use default navigation within the Recently Used group.
				m = moveSelectionUp(m)
			}
		}
		// Update preview based on new selection
		m = updatePreview(m)

	case "down", "j":
		if group == "action" {
			// If in the Action Row, press down to jump to the top of the Recently Used group.
			if recentCount > 0 {
				m.SelectedIndex = actionCount // top of recent group
			} else {
				m.SelectedIndex = 0
			}
		} else { // group == "recent"
			// If at the bottom of the Recently Used group, jump to the previously highlighted Action Row item.
			if m.SelectedIndex == actionCount+recentCount-1 {
				if m.LastActionIndex >= 0 && m.LastActionIndex < actionCount {
					m.SelectedIndex = m.LastActionIndex
				} else {
					m.SelectedIndex = 0
				}
			} else {
				// Otherwise, use default navigation within Recently Used.
				m = moveSelectionDown(m)
			}
		}
		// Update preview based on new selection
		m = updatePreview(m)

	case "enter":
		itemName, _ := getItemName(m, m.SelectedIndex)
		// Special handling for "view project stats" on Enter
		if strings.ToLower(itemName) == "view project stats" {
			m.CurrentScreen = app.ScreenProjectStats
			// Clear previews when navigating away
			m.CurrentPreviewType = "none"
			m.FileTreePreview = ""
			m.StatsPreview = ""
			return m, nil // Return early
		}

		if strings.ToLower(itemName) == "paste from clipboard" {
			m.PendingCommand = itemName

			// Check if clipboard content requires multiple variables
			if requiresMultipleVars(itemName) {
				m.MultipleVariables = true
				m.VariableKeys = extractVariableKeys(itemName)
				fmt.Printf("Detected clipboard variable keys: %v\n", m.VariableKeys)
				m.CurrentVariableIndex = 0
				m.Variables = make(map[string]string)
			}

			m.CurrentScreen = app.ScreenFilenamePrompt
			m.TempFilename = ""
			// Clear previews when navigating away
			m.CurrentPreviewType = "none"
			m.FileTreePreview = ""
			m.StatsPreview = ""
		} else {
			m = *HandleCommandSelection(&m, itemName)
			// Clear previews after command execution starts
			m.CurrentPreviewType = "none"
			m.FileTreePreview = ""
			m.StatsPreview = ""
		}
	}
	return m, nil
}

// NEW: updatePreview determines and generates the correct preview based on the current selection.
func updatePreview(m app.Model) app.Model {
	cmdName, _ := getItemName(m, m.SelectedIndex)
	lowerCmd := strings.ToLower(cmdName)

	// Reset previews
	m.FileTreePreview = ""
	m.StatsPreview = ""
	m.CurrentPreviewType = "none"

	switch lowerCmd {
	case "view project stats":
		m.StatsPreview = app.SummarizeFullProjectStats(m.RecognizedPkgs)
		if m.StatsPreview == "" {
			m.StatsPreview = "No project stats available."
		}
		m.CurrentPreviewType = "stats"
	case "undo", "redo":
		// No preview for these actions
		m.CurrentPreviewType = "none"
	case "paste from clipboard":
		// Generate preview based on clipboard content
		// Use default placeholders as we don't have real input yet
		placeholderMap := commands.BuildAutoPlaceholders(map[string]string{"Filename": "<PastedItem>"})
		pv, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath)
		if err == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		} else {
			m.FileTreePreview = "Preview unavailable for clipboard content."
			if err != nil {
				m.FileTreePreview += fmt.Sprintf("\nError: %v", err)
			}
			m.CurrentPreviewType = "none" // Set to none if preview failed
		}
	default:
		// Attempt to generate file tree preview for other commands
		spec := commands.GetCommandSpec(cmdName)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			// Use the first key as the primary placeholder
			placeholders := map[string]string{keys[0]: "<" + keys[0] + ">"}
			placeholderMap = commands.BuildPlaceholders(placeholders)
		} else {
			// Fallback if no keys found or error
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Filename>"})
		}

		pv, err2 := commands.GeneratePreviewFileTree(cmdName, placeholderMap, m.ProjectPath)
		if err2 == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		} else {
			m.CurrentPreviewType = "none"
			if err2 != nil {
				// Optional: Log error: fmt.Printf("Preview error for %s: %v\n", cmdName, err2)
			}
		}
	}
	return m
}

// ViewMainScreen is the view for the main screen.
func ViewMainScreen(m app.Model) string {
	// Logo logic: "NEXTGEN CLI" where "GEN" is styled with color "#ff3600".
	logo := app.TitleStyle.Render("NEXT") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("GEN") +
		" CLI" + lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(" v0.01") + "\n"
	title := logo

	// Gray path row

	// Start building body using the new logo title.
	body := title

	// Display grouped package recognizer stats.
	// (This now groups React-based frameworks and aggregates CSS frameworks.)
	body += app.SummarizeProjectStats(m.RecognizedPkgs) + "\n"

	// Render the Action Row at the top (icons only):

	body += renderActionRowItems(actionRow, &m, 0, len(actionRow))

	// Render Recently Used commands below the Action Row:
	body += "\n" + app.SubtitleStyle.Render("Recently used commands:") + "\n\n"
	recentFiltered := filterRecentUsed(commands.RecentUsed)
	// Note: Offset is now len(actionRow) because the merged ordering is actionRow first.
	body += renderRecentUsedInColumns(recentFiltered, &m, len(actionRow), 1, 10)

	body += "\n" + app.HelpStyle.Render("(Use arrow keys or j/k to move; q quits.)")

	// Build the left panel (the main Recent Commands view).
	leftPanel := baseContainer(body)

	// --- Render the dynamic preview based on the model state ---
	var previewContent string
	switch m.CurrentPreviewType {
	case "stats":
		previewContent = m.StatsPreview
	case "file-tree":
		previewContent = m.FileTreePreview
	default:
		previewContent = "No preview available for this command."
	}

	// Truncate the preview to fit reasonably within the available height.
	// Use TerminalHeight for a more robust calculation.
	// Subtracting a fixed number accounts for headers, footers, padding etc.
	maxPreviewHeight := m.TerminalHeight - 10 // Adjust this offset as needed
	if maxPreviewHeight < 5 {                 // Ensure a minimum height
		maxPreviewHeight = 5
	}
	lpHeight := lipgloss.Height(leftPanel)
	if lpHeight > maxPreviewHeight { // Ensure preview isn't taller than calculated max
		maxPreviewHeight = lpHeight
	}

	lines := strings.Split(previewContent, "\n")
	if len(lines) > maxPreviewHeight {
		previewContent = strings.Join(lines[:maxPreviewHeight], "\n")
		previewContent += "\n... (truncated)"
	}

	// Prepend header with package icon and current folder name.
	folderName := filepath.Base(m.ProjectPath)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContent = header + "\n" + previewContent

	// Build the right panel (the preview panel).
	rightPanel := baseContainer(previewContent)

	// Combine left and right panels using lipgloss.JoinHorizontal.
	// Use Top alignment and add some space between panels.
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	// Optional: Add a footer with general help or status.
	// footer := lipgloss.NewStyle().MarginTop(1).Render("Press 'q' to quit.")
	// return combined + "\n" + footer

	return combined
}

// renderRecentUsedInColumns displays recent commands in *column-major* order, filling each column top-down.
// Added icons by calling app.CommandWithIcon(cmd).
func renderRecentUsedInColumns(items []string, m *app.Model, offset, columns, rows int) string {
	colStyle := lipgloss.NewStyle().
		Width(40).
		MarginRight(2).
		Align(lipgloss.Left)

	var out string
	for row := 0; row < rows; row++ {
		line := ""
		for col := 0; col < columns; col++ {
			index := col*rows + row
			if index >= len(items) {
				break
			}
			fullIndex := offset + index
			cmd := items[index]

			// Use the icon for each command
			iconCmd := commands.CommandWithIcon(cmd)

			// Highlight the selected item without using > < markers
			if m.SelectedIndex == fullIndex && m.CurrentScreen == app.ScreenMain {
				line += colStyle.Render(app.HighlightStyle.Render(iconCmd))
			} else {
				line += colStyle.Render(app.ChoiceStyle.Render(iconCmd))
			}
		}
		if line != "" {
			out += line + "\n"
		}
	}
	return out
}

// moveSelectionUp moves one item up over the entire navigable list.
func moveSelectionUp(m app.Model) app.Model {
	total := totalNavCount()
	if total == 0 {
		return m
	}
	if m.SelectedIndex > 0 {
		m.SelectedIndex--
	} else {
		m.SelectedIndex = total - 1
	}
	return m
}

// moveSelectionDown moves one item down over the entire navigable list.
func moveSelectionDown(m app.Model) app.Model {
	total := totalNavCount()
	if total == 0 {
		return m
	}
	if m.SelectedIndex < total-1 {
		m.SelectedIndex++
	} else {
		m.SelectedIndex = 0
	}
	return m
}

// UpdateScreenProjectStats handles input on the Project Stats screen.
// It returns to the main screen on any key press.
func UpdateScreenProjectStats(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	m.CurrentScreen = app.ScreenMain
	return m, nil
}

// ViewProjectStatsScreen renders the full project stats (all recognized packages)
// along with a header and footer.
// For the full version that includes project registry data, use ViewProjectStatsScreenWithRegistry.
func ViewProjectStatsScreen(m app.Model) string {
	// This is a backward-compatible wrapper for apps that don't yet have access to the registry
	return ViewProjectStatsScreenWithRegistry(m, nil)
}

// ViewProjectStatsScreenWithRegistry renders project stats with additional information from the
// project registry if available.
func ViewProjectStatsScreenWithRegistry(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Project Stats") + "\n\n"

	// Display project path if available
	var body string
	if m.ProjectPath != "" {
		body += app.SubtitleStyle.Render("Project Path: ") +
			app.PathStyle.Render(m.ProjectPath) + "\n\n"
	}

	// Display packages
	if len(m.RecognizedPkgs) > 0 {
		body += app.SubtitleStyle.Render("Detected Packages:") + "\n"
		body += app.SummarizeFullProjectStats(m.RecognizedPkgs) + "\n"
	} else {
		body += app.ChoiceStyle.Render("No packages detected") + "\n\n"
	}

	// Add project usage information from registry if available
	body += app.SubtitleStyle.Render("Project Usage:") + "\n"

	if registry != nil && m.ProjectPath != "" {
		// Try to get project info from registry
		if projectInfo, found := registry.GetProject(m.ProjectPath); found {
			// Format the usage count
			body += app.ChoiceStyle.Render("- Usage Count: ") +
				fmt.Sprintf("%d", projectInfo.UsageCount) + "\n"

			// Format the last access time
			lastAccess := time.Unix(projectInfo.LastAccessTime, 0)
			body += app.ChoiceStyle.Render("- Last Access: ") +
				lastAccess.Format("Jan 2, 2006 at 3:04 PM") + "\n"

			// Add project type if available
			if projectInfo.Type != "" {
				body += app.ChoiceStyle.Render("- Project Type: ") +
					projectInfo.Type + "\n"
			}
		} else {
			body += app.ChoiceStyle.Render("- Usage Count: ") + "Not yet recorded\n"
			body += app.ChoiceStyle.Render("- Last Access: ") + "Not yet recorded\n"
		}
	} else {
		body += app.ChoiceStyle.Render("- Usage Count: ") + "Not available\n"
		body += app.ChoiceStyle.Render("- Last Access: ") + "Not available\n"
	}

	body += "\n"

	// Add placeholder for command history information
	body += app.SubtitleStyle.Render("Command History:") + "\n"

	if registry != nil {
		body += app.ChoiceStyle.Render("- Total Global CLI Usage: ") +
			fmt.Sprintf("%d", registry.GlobalUsages) + "\n"
	} else {
		body += app.ChoiceStyle.Render("- Most Used Command: ") + "Not yet available\n"
		body += app.ChoiceStyle.Render("- Total Commands Run: ") + "Not yet available\n"
	}

	body += "\n"

	footer := app.HelpStyle.Render("Press any key to return to main screen")
	return header + body + "\n" + footer
}

// renderActionRowItems displays the given items in a row-based layout but only shows their icons.
func renderActionRowItems(items []string, m *app.Model, offset, columns int) string {
	var outputLines []string
	var currentLine string

	for i, val := range items {
		if i != 0 && i%columns == 0 {
			outputLines = append(outputLines, currentLine)
			currentLine = ""
		}

		// Determine the icon for the action.
		lowerVal := strings.ToLower(val)
		var icon string
		switch lowerVal {
		case "undo":
			icon = "↺"
		case "redo":
			icon = "↻"
		case "paste from clipboard":
			icon = "📋"
		case "view project stats":
			icon = "📦"
		default:
			icon = val // fallback to text if not recognized
		}

		fullIndex := offset + i
		if m.SelectedIndex == fullIndex && m.CurrentScreen == app.ScreenMain {
			currentLine += app.HighlightStyle.Render("> "+icon+" <") + "  "
		} else {
			currentLine += app.ChoiceStyle.Render(icon) + "  "
		}
	}

	if currentLine != "" {
		outputLines = append(outputLines, currentLine)
	}

	return strings.Join(outputLines, "\n") + "\n"
}
