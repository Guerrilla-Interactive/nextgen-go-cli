// cmd/screens/helpers.go
package screens

import (
	"strings"
)

// summarizeProjectStats is a small example that returns a string with project stats.
func summarizeProjectStats(m myapp.model) string {
	result := myapp.pathStyle.Render(m.ProjectPath) + "\n\n"
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
			// Join horizontally
			lines = append(lines, strings.Join(currentLine, " | "))
			currentLine = nil
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " | "))
	}

	return strings.Join(lines, "\n") + "\n"
}

// getItemName returns the label (and a boolean if it's the last item).
func getItemName(m myapp.model, index int) (string, bool) {
	offset := len(myapp.recentUsed) + (len(myapp.nextSteps) - 1)
	if index == offset {
		if m.IsLoggedIn {
			return "Logout", true
		}
		return "Login", true
	}
	if index < len(myapp.recentUsed) {
		return myapp.recentUsed[index], false
	}
	stepIndex := index - len(myapp.recentUsed)
	return myapp.nextSteps[stepIndex], false
}

// recordCommand moves the chosen command to the front of recentUsed, removing duplicates, limit to 8.
func recordCommand(m *myapp.model, cmd string) {
	idx := -1
	for i, v := range myapp.recentUsed {
		if v == cmd {
			idx = i
			break
		}
	}
	if idx != -1 {
		myapp.recentUsed = append(myapp.recentUsed[:idx], myapp.recentUsed[idx+1:]...)
	}
	myapp.recentUsed = append([]string{cmd}, myapp.recentUsed...)

	if len(myapp.recentUsed) > 8 {
		myapp.recentUsed = myapp.recentUsed[:8]
	}

	m.TotalItems = len(myapp.recentUsed) + len(myapp.nextSteps)
}

func renderItemsHorizontally(items []string, m *myapp.model, offset, columns int) string {
	var outputLines []string
	var currentLine string

	for i, val := range items {
		if i != 0 && i%columns == 0 {
			outputLines = append(outputLines, currentLine)
			currentLine = ""
		}

		fullIndex := offset + i
		if m.SelectedIndex == fullIndex && m.CurrentScreen == myapp.screenMain {
			currentLine += myapp.highlightStyle.Render("> "+val+" <") + "  "
		} else {
			currentLine += myapp.choiceStyle.Render(val) + "  "
		}
	}
	if currentLine != "" {
		outputLines = append(outputLines, currentLine)
	}

	return strings.Join(outputLines, "\n") + "\n"
}

// renderItemList is used for the nextSteps on the main screen.
func renderItemList(items []string, m *myapp.model, offset int) string {
	var out string
	for i, val := range items {
		fullIndex := offset + i
		if m.SelectedIndex == fullIndex && m.CurrentScreen == myapp.screenMain {
			out += myapp.highlightStyle.Render("> "+val+" <") + "\n"
		} else {
			out += myapp.choiceStyle.Render(val) + "\n"
		}
	}
	return out
}
