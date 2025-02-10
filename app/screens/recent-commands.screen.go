package screens

import (
	"fmt"
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenMain handles input for the main screen with "smart" arrow navigation:
//   - 3 columns × 5 rows in column-major order for RecentUsed.
//   - Pressing ↓ on the bottom row goes to the first NextSteps item ("Show all my commands");
//     pressing ↓ again goes to "Back"; pressing ↓ again wraps to the top of RecentUsed.
//   - Pressing ↑ on the top row goes to the last NextSteps item ("Back") if it exists; pressing ↑ again
//     moves to the first NextSteps, pressing ↑ again returns to the bottom of RecentUsed.
//   - SPECIAL REQUEST: When ↑ from the first NextSteps item ("Show all my commands"),
//     select the bottom of the first column in RecentUsed (index=4 if we have ≥5 commands).
func UpdateScreenMain(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "left", "h":
		m = moveSelectionLeft(m)

	case "right", "l":
		m = moveSelectionRight(m)

	case "up", "k":
		m = moveSelectionUp(m)

	case "down", "j":
		m = moveSelectionDown(m)

	case "enter":
		itemName, isLast := getItemName(m, m.SelectedIndex)
		if isLast {
			m.IsLoggedIn = !m.IsLoggedIn
			m.CurrentScreen = app.ScreenSelect
		} else {
			m = *HandleCommandSelection(&m, itemName)
		}
	}
	return m, nil
}

// ViewMainScreen is the view for the main screen.
func ViewMainScreen(m app.Model) string {
	// Title logic
	titleText := "=== Offline Mode ==="
	if m.IsLoggedIn {
		titleText = "=== Online Mode ==="
	}
	title := app.TitleStyle.Render(titleText)

	// Gray path row
	pathLine := app.PathStyle.Render(m.ProjectPath)

	// Start building body
	body := title + "\n" + pathLine + "\n\n"

	// Optionally (if you still want to display recognized packages info):
	body += summarizeProjectStats(m) + "\n"

	body += app.SubtitleStyle.Render("Recent used commands:") + "\n\n"
	// 3×5 grid (column-major):
	body += renderRecentUsedInColumns(commands.RecentUsed, &m, 0, 3, 5)

	body += "\n"

	// NextSteps: [ "Show all my commands", "Back" ]
	// We'll rename the second item to "Back" for the user:
	opts := []string{commands.NextSteps[0], "Back"}
	body += renderItemList(opts, m, len(commands.RecentUsed))

	body += "\n" + app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)")

	// Build the left panel (the main Recent Commands view).
	leftPanel := baseContainer(body)

	// Build a live preview for the currently selected command.
	var preview string
	if len(commands.RecentUsed) > 0 && m.SelectedIndex < len(commands.RecentUsed) {
		// Get the currently selected command name.
		cmdName := commands.RecentUsed[m.SelectedIndex]
		// Retrieve the command spec and its template variable keys.
		spec := commands.GetCommandSpec(cmdName)
		keys, err := commands.GetTemplateVariableKeys(spec)
		var placeholderMap map[string]string
		// Use the first key (if any) to build the placeholder map.
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: "Filename"})
		} else {
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "Filename"})
		}
		// Generate the preview file tree.
		pv, err2 := commands.GeneratePreviewFileTree(cmdName, placeholderMap, m.ProjectPath)
		if err2 == nil {
			preview = pv
		} else {
			preview = fmt.Sprintf("Preview unavailable: %v", err2)
		}
	} else {
		preview = "No command selected for preview."
	}
	// Build the right panel (the preview view).
	rightPanel := baseContainer(preview)

	// Join the left and right panels horizontally.
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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

// moveSelectionLeft moves the selection one column to the left (if possible).
func moveSelectionLeft(m app.Model) app.Model {
	totalUsed := len(commands.RecentUsed)
	if m.SelectedIndex >= totalUsed {
		return m // In NextSteps, left does nothing
	}

	const columns = 3
	const rows = 5

	col := m.SelectedIndex / rows
	row := m.SelectedIndex % rows
	if col > 0 {
		col--
	}
	newIdx := col*rows + row
	if newIdx < totalUsed {
		m.SelectedIndex = newIdx
	}
	return m
}

// moveSelectionRight moves the selection one column to the right (if possible).
func moveSelectionRight(m app.Model) app.Model {
	totalUsed := len(commands.RecentUsed)
	if m.SelectedIndex >= totalUsed {
		return m // In NextSteps, right does nothing
	}

	const columns = 3
	const rows = 5

	col := m.SelectedIndex / rows
	row := m.SelectedIndex % rows
	if col < columns-1 {
		col++
	}
	newIdx := col*rows + row
	if newIdx < totalUsed {
		m.SelectedIndex = newIdx
	}
	return m
}

// moveSelectionUp handles upward movement:
// If in NextSteps, move up among them or wrap back onto the bottom row of RecentUsed.
// If in top row of RecentUsed, jump to the last NextSteps item; else, just row--.
// SPECIAL: If up from the first NextStep, go to the bottom of the first column (index=4 if >=5 commands).
func moveSelectionUp(m app.Model) app.Model {
	totalUsed := len(commands.RecentUsed)
	if totalUsed == 0 {
		return m // no recent items
	}

	const columns = 3
	const rows = 5

	// If in NextSteps:
	if m.SelectedIndex >= totalUsed {
		stepIndex := m.SelectedIndex - totalUsed
		// If at the first NextStep => jump to bottom of the first column
		if stepIndex == 0 {
			if totalUsed >= 5 {
				m.SelectedIndex = 4 // bottom row of the first column
			} else {
				// If fewer than 5 commands exist, just go to the last command we have
				m.SelectedIndex = totalUsed - 1
			}
			return m
		}
		// Otherwise, move up within NextSteps
		stepIndex--
		m.SelectedIndex = totalUsed + stepIndex
		return m
	}

	// In RecentUsed:
	col := m.SelectedIndex / rows
	row := m.SelectedIndex % rows
	if row == 0 {
		// If in top row, jump to the last next step (if it exists), otherwise the first
		if len(commands.NextSteps) > 1 {
			m.SelectedIndex = totalUsed + 1 // second next step => "Back"
		} else {
			m.SelectedIndex = totalUsed // first next step
		}
		return m
	}

	// Otherwise, just move up a row
	row--
	m.SelectedIndex = col*rows + row
	return m
}

// moveSelectionDown handles downward movement:
// If on bottom row of RecentUsed, move to first NextStep; then second NextStep; then wrap to top, etc.
func moveSelectionDown(m app.Model) app.Model {
	totalUsed := len(commands.RecentUsed)
	const columns = 3
	const rows = 5

	// If no RecentUsed, skip directly to NextSteps logic
	if totalUsed == 0 {
		return moveSelectionDownInNextSteps(m)
	}

	// If already in NextSteps:
	if m.SelectedIndex >= totalUsed {
		return moveSelectionDownInNextSteps(m)
	}

	// Otherwise, we're in RecentUsed
	col := m.SelectedIndex / rows
	row := m.SelectedIndex % rows

	// If on bottom row, jump to the first NextStep
	if row == rows-1 {
		m.SelectedIndex = totalUsed // index of the first NextStep item
		return m
	}

	// Move down one row
	row++
	newIdx := col*rows + row
	if newIdx >= totalUsed {
		// If there's no item there, go to the first NextStep
		m.SelectedIndex = totalUsed
	} else {
		m.SelectedIndex = newIdx
	}
	return m
}

// moveSelectionDownInNextSteps moves the selection down among NextSteps.
// If we pass the last NextStep, wrap to the top of RecentUsed (index=0).
func moveSelectionDownInNextSteps(m app.Model) app.Model {
	totalUsed := len(commands.RecentUsed)
	stepIndex := m.SelectedIndex - totalUsed
	stepIndex++
	if stepIndex >= len(commands.NextSteps) {
		// Wrap back to the top of RecentUsed
		m.SelectedIndex = 0
	} else {
		m.SelectedIndex = totalUsed + stepIndex
	}
	return m
}
