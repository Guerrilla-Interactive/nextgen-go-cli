package screens

import (
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/charmbracelet/lipgloss"
)

// summarizeProjectStats returns a string with project stats.
func summarizeProjectStats(m app.Model) string {
	result := app.PathStyle.Render(m.ProjectPath) + "\n\n"
	if len(m.RecognizedPkgs) == 0 {
		result += "    • None recognized packages\n"
	} else {
		// Render recognized packages in up to 6 columns using Lipgloss.
		result += renderPackagesHorizontally(m.RecognizedPkgs, 6)
	}
	return result
}

// renderPackagesHorizontally displays items in a grid of fixed-width columns, up to maxCols columns.
func renderPackagesHorizontally(items []string, maxCols int) string {
	if len(items) == 0 {
		return ""
	}

	// Number of columns is either maxCols or fewer if we have fewer items.
	cols := maxCols
	if len(items) < cols {
		cols = len(items)
	}
	// Compute how many rows we need
	rows := (len(items) + cols - 1) / cols // integer ceiling

	// Define a lipgloss style for fixed-width columns.
	// Adjust width to your preference.
	colStyle := lipgloss.NewStyle().
		Width(18).
		MarginRight(2).
		Align(lipgloss.Left)

	var lines []string

	// Render items in rows x cols layout
	for r := 0; r < rows; r++ {
		var line string
		for c := 0; c < cols; c++ {
			index := c*rows + r
			if index >= len(items) {
				break
			}
			item := items[index]
			line += colStyle.Render(item)
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

// getItemName returns the label (and a bool if it's the last item).
func getItemName(m app.Model, index int) (string, bool) {
	// offset = len(RecentUsed) + (len(NextSteps) - 1)
	offset := len(app.RecentUsed) + (len(app.NextSteps) - 1)
	if index == offset {
		if m.IsLoggedIn {
			return "Logout", true
		}
		return "Login", true
	}
	if index < len(app.RecentUsed) {
		return app.RecentUsed[index], false
	}
	stepIndex := index - len(app.RecentUsed)
	return app.NextSteps[stepIndex], false
}

// recordCommand moves the chosen command to the front of RecentUsed, removing duplicates, limit to 8.
func recordCommand(m *app.Model, cmd string) {
	idx := -1
	for i, v := range app.RecentUsed {
		if v == cmd {
			idx = i
			break
		}
	}
	if idx != -1 {
		// Remove it from old position
		app.RecentUsed = append(app.RecentUsed[:idx], app.RecentUsed[idx+1:]...)
	}
	// Add to front
	app.RecentUsed = append([]string{cmd}, app.RecentUsed...)

	// Cap at 8
	if len(app.RecentUsed) > 8 {
		app.RecentUsed = app.RecentUsed[:8]
	}

	m.TotalItems = len(app.RecentUsed) + len(app.NextSteps)
}

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
