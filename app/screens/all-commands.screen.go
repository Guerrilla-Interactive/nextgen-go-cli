package screens

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenAll handles keypresses on the “all commands” screen with “smart” arrow navigation.
// We treat each column as having up to 10 commands, and there may be multiple columns.
// Pressing ↓ from the bottom row goes to "Back"; pressing ↓ again while on "Back" wraps to the top (index=0).
// Pressing ↑ from the top row goes to "Back"; pressing ↑ again while on "Back" ⇒ go to bottom of the first column.
func UpdateScreenAll(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		os.Exit(0)

	case "left", "h":
		m = moveAllCmdsSelectionLeft(m)

	case "right", "l":
		m = moveAllCmdsSelectionRight(m)

	case "up", "k":
		m = moveAllCmdsSelectionUp(m)

	case "down", "j":
		m = moveAllCmdsSelectionDown(m)

	case "enter":
		// If on "Back" index, go back to main screen
		if m.AllCmdsIndex == len(app.AllCommands) {
			m.CurrentScreen = app.ScreenMain
		} else {
			cmd := app.AllCommands[m.AllCmdsIndex]
			recordCommand(&m, cmd)
		}
	}
	return m, nil
}

// ViewAllScreen renders the “all commands” screen in pretty, fixed-width columns.
func ViewAllScreen(m app.Model) string {
	// Title
	title := app.TitleStyle.Render("=== All Commands ===")
	// Gray path row
	pathLine := app.PathStyle.Render(m.ProjectPath)

	// Start body
	body := title + "\n" + pathLine + "\n\n"
	body += app.SubtitleStyle.Render("Select a command (Enter to log usage).") + "\n\n"

	commands := app.AllCommands
	commandsCount := len(commands)
	const rows = 10
	columns := (commandsCount + rows - 1) / rows

	// Fixed-width columns
	const colWidth = 30
	colStyle := lipgloss.NewStyle().
		Width(colWidth).
		MarginRight(2).
		Align(lipgloss.Left)

	// Render commands in column-major order
	for r := 0; r < rows; r++ {
		line := ""
		for c := 0; c < columns; c++ {
			idx := c*rows + r
			if idx >= commandsCount {
				break
			}
			cmd := commands[idx]
			if m.AllCmdsIndex == idx {
				line += colStyle.Render(app.HighlightStyle.Render("> " + cmd + " <"))
			} else {
				line += colStyle.Render(app.ChoiceStyle.Render(cmd))
			}
		}
		if line != "" {
			body += line + "\n"
		}
	}

	// Provide "Back" option
	if m.AllCmdsIndex == commandsCount {
		body += "\n" + app.HighlightStyle.Render("> Back <") + "\n"
	} else {
		body += "\n" + app.ChoiceStyle.Render("Back") + "\n"
	}

	body += "\n" + app.HelpStyle.Render("(Use arrows or j/k/up/down to move; Enter on 'Back' returns to main screen; q quits.)")

	return body
}

// moveAllCmdsSelectionLeft moves the selection one column to the left (if possible).
func moveAllCmdsSelectionLeft(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(app.AllCommands)
	if idx == commandsCount {
		// On "Back," do nothing
		return m
	}

	const rows = 10
	col := idx / rows
	row := idx % rows

	if col > 0 {
		col--
	}
	newIdx := col*rows + row
	// Make sure it's valid
	if newIdx < commandsCount {
		m.AllCmdsIndex = newIdx
	}
	return m
}

// moveAllCmdsSelectionRight moves the selection one column to the right (if possible).
func moveAllCmdsSelectionRight(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(app.AllCommands)
	if idx == commandsCount {
		// On "Back," do nothing
		return m
	}

	const rows = 10
	columns := (commandsCount + rows - 1) / rows

	col := idx / rows
	row := idx % rows

	if col < columns-1 {
		col++
	}
	newIdx := col*rows + row
	if newIdx >= commandsCount {
		// If that column doesn't have a row at “row,” do nothing
		return m
	}
	m.AllCmdsIndex = newIdx
	return m
}

// moveAllCmdsSelectionUp moves the selection one row up.
// If on "Back," go to bottom of the first column. If already on top row, move to "Back."
func moveAllCmdsSelectionUp(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(app.AllCommands)
	if idx == commandsCount {
		// If on Back, go to bottom of first column (index=rows-1), unless we have fewer items
		const rows = 10
		if commandsCount > 0 {
			bottomFirstCol := rows - 1
			if bottomFirstCol >= commandsCount {
				bottomFirstCol = commandsCount - 1
			}
			m.AllCmdsIndex = bottomFirstCol
		}
		return m
	}

	const rows = 10
	col := idx / rows
	row := idx % rows

	if row == 0 {
		// Move to "Back"
		m.AllCmdsIndex = commandsCount
		return m
	}

	row--
	newIdx := col*rows + row
	m.AllCmdsIndex = newIdx
	return m
}

// moveAllCmdsSelectionDown moves the selection one row down.
// If on the bottom row, go to "Back."
// If on "Back" and press down, wrap to top (index=0).
func moveAllCmdsSelectionDown(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(app.AllCommands)
	if idx == commandsCount {
		// If on Back, wrap to top
		m.AllCmdsIndex = 0
		return m
	}

	const rows = 10
	col := idx / rows
	row := idx % rows

	if row == rows-1 {
		// If on bottom row, jump to "Back"
		m.AllCmdsIndex = commandsCount
		return m
	}

	row++
	newIdx := col*rows + row
	if newIdx >= commandsCount {
		// If there's no item below in that column, jump to "Back"
		m.AllCmdsIndex = commandsCount
	} else {
		m.AllCmdsIndex = newIdx
	}
	return m
}
