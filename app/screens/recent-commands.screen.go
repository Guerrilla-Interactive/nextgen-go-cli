package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
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

	case "enter":
		itemName, _ := getItemName(m, m.SelectedIndex)
		if strings.ToLower(itemName) == "paste from clipboard" {
			m.PendingCommand = itemName
			m.CurrentScreen = app.ScreenFilenamePrompt
			m.TempFilename = ""
			m.LivePreview = ""
		} else {
			m = *HandleCommandSelection(&m, itemName)
		}
	}
	return m, nil
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

	// Build the live preview for the currently selected command using the merged navigation index.
	cmdName, _ := getItemName(m, m.SelectedIndex)
	var preview string
	lowerCmd := strings.ToLower(cmdName)
	// Only attempt preview for commands that are not navigation or action commands.
	if lowerCmd != "undo" && lowerCmd != "redo" &&
		lowerCmd != "show all my commands" && lowerCmd != "view project stats" {
		// Retrieve the command spec and its template variable keys.
		spec := commands.GetCommandSpec(cmdName)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: "Filename"})
		} else {
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "Filename"})
		}
		var pv string
		var err2 error
		if lowerCmd == "paste from clipboard" {
			pv, err2 = commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath)
		} else {
			pv, err2 = commands.GeneratePreviewFileTree(cmdName, placeholderMap, m.ProjectPath)
		}
		if err2 == nil {
			preview = pv
		} else {
			preview = fmt.Sprintf("Preview unavailable: %v", err2)
		}
	} else {
		preview = "No preview available for this command."
	}
	// Truncate the preview so it is shorter than the left panel.
	lpHeight := lipgloss.Height(leftPanel)
	maxPreviewHeight := lpHeight // Adjust this expression if needed, e.g. (lpHeight + 100) / 2
	if maxPreviewHeight < 1 {
		maxPreviewHeight = 1
	}
	lines := strings.Split(preview, "\n")
	if len(lines) > maxPreviewHeight {
		preview = strings.Join(lines[:maxPreviewHeight], "\n")
	}

	// Prepend header with package icon and current folder name.
	folderName := filepath.Base(m.ProjectPath)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	preview = header + "\n" + preview

	// Declare the rightPanel variable.
	rightPanel := sideContainer(preview)

	// Use a fallback if TerminalHeight is zero.
	termHeight := m.TerminalHeight
	if termHeight == 0 {
		termHeight = 24
	}

	// Force the left panel to have a fixed height equal to termHeight and align its content to the bottom.
	fixedLeftPanel := lipgloss.Place(
		lipgloss.Width(leftPanel), // preserve left panel width
		termHeight,                // fixed height
		lipgloss.Left,             // horizontal alignment
		lipgloss.Bottom,           // vertical alignment (bottom)
		leftPanel,                 // content to anchor
		lipgloss.WithWhitespaceChars(" "),
	)

	// Anchor the right panel (the preview/tree) to the bottom as well.
	anchoredRightPanel := lipgloss.Place(
		lipgloss.Width(rightPanel), // preserve right panel width
		termHeight,                 // fixed height
		lipgloss.Left,              // horizontal alignment
		lipgloss.Bottom,            // vertical alignment (bottom)
		rightPanel,                 // content to anchor
	)

	// Join the anchored panels horizontally.
	return lipgloss.JoinHorizontal(lipgloss.Bottom, fixedLeftPanel, anchoredRightPanel)
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
// On any key press it returns to the Main screen.
func UpdateScreenProjectStats(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	m.CurrentScreen = app.ScreenMain
	return m, nil
}

// ViewProjectStatsScreen renders the full project stats (all recognized packages)
// along with a header and footer.
func ViewProjectStatsScreen(m app.Model) string {
	header := app.TitleStyle.Render("Project Stats") + "\n\n"
	body := app.SummarizeFullProjectStats(m.RecognizedPkgs) + "\n"
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
