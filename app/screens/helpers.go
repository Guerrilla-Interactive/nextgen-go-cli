package screens

import (
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
)

// summarizeProjectStats returns a string with project stats.
func summarizeProjectStats(m app.Model) string {
	result := app.PathStyle.Render(m.ProjectPath) + "\n\n"
	if len(m.RecognizedPkgs) == 0 {
		result += "    • None recognized packages\n"
	} else {
		// E.g. 6 columns max
		result += renderPackagesHorizontally(m.RecognizedPkgs, 6)
	}
	return result
}

func renderPackagesHorizontally(items []string, columns int) string {
	var lines []string
	var currentLine []string

	for i, pkg := range items {
		currentLine = append(currentLine, pkg)
		if (i+1)%columns == 0 {
			lines = append(lines, strings.Join(currentLine, " | "))
			currentLine = nil
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " | "))
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
func renderItemList(items []string, m *app.Model, offset int) string {
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
