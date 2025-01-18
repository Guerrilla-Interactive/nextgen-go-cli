package screens

import (
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
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
// without a fixed width. Now we place a bullet (•) between each package in a row.
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
