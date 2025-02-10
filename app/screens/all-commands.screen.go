package screens

import (
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateScreenAll handles keypresses on the "all commands" screen with "smart" arrow navigation.
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
		all := commands.AllCommandNames()
		if m.AllCmdsIndex == len(all) {
			m.CurrentScreen = app.ScreenMain
		} else {
			cmdName := all[m.AllCmdsIndex]
			if cmdName == "add page" {
				m.PendingCommand = "add page"
				m.CurrentScreen = app.ScreenFilenamePrompt
			} else {
				recordCommand(&m, cmdName)
				return m, func() tea.Msg {
					err := commands.RunCommand(cmdName, m.ProjectPath, nil)
					return CommandFinishedMsg{Err: err}
				}
			}
		}
	}
	return m, nil
}

// ViewAllScreen renders the "all commands" screen in pretty, fixed-width columns,
// now including icons for each command and removing the > < markers.
func ViewAllScreen(m app.Model) string {
	// Title
	title := app.TitleStyle.Render("=== All Commands ===")
	// Gray path row
	pathLine := app.PathStyle.Render(m.ProjectPath)

	// Start body
	body := title + "\n" + pathLine + "\n\n"
	body += app.SubtitleStyle.Render("Select a command (Enter to log usage).") + "\n\n"

	all := commands.AllCommandNames()
	commandsCount := len(all)
	const rows = 10
	columns := (commandsCount + rows - 1) / rows

	// Fixed-width columns
	const colWidth = 40
	colStyle := lipgloss.NewStyle().
		Width(colWidth).
		MarginRight(2).
		Align(lipgloss.Left)

	// Render commands in column-major order, using icons
	for r := 0; r < rows; r++ {
		line := ""
		for c := 0; c < columns; c++ {
			idx := c*rows + r
			if idx >= commandsCount {
				break
			}
			cmdName := all[idx]
			cmdWithIcon := commands.CommandWithIcon(cmdName)

			if m.AllCmdsIndex == idx {
				line += colStyle.Render(app.HighlightStyle.Render(cmdWithIcon))
			} else {
				line += colStyle.Render(app.ChoiceStyle.Render(cmdWithIcon))
			}
		}
		if line != "" {
			body += line + "\n"
		}
	}

	// Provide "Back" option (highlight if selected, no > <).
	if m.AllCmdsIndex == commandsCount {
		body += "\n" + app.HighlightStyle.Render("Back") + "\n"
	} else {
		body += "\n" + app.ChoiceStyle.Render("Back") + "\n"
	}

	body += "\n" + app.HelpStyle.Render("(Use arrows or j/k/up/down to move; Enter on 'Back' returns to main screen; q quits.)")

	return baseContainer(body)
}

// moveAllCmdsSelectionLeft moves the selection one column to the left (if possible).
func moveAllCmdsSelectionLeft(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(commands.AllCommandNames())
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
	commandsCount := len(commands.AllCommandNames())
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
		// If that column doesn't have a row at "row," do nothing
		return m
	}
	m.AllCmdsIndex = newIdx
	return m
}

// moveAllCmdsSelectionUp moves the selection one row up.
// If on "Back," go to bottom of the first column. If already on top row, move to "Back."
func moveAllCmdsSelectionUp(m app.Model) app.Model {
	idx := m.AllCmdsIndex
	commandsCount := len(commands.AllCommandNames())
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
	commandsCount := len(commands.AllCommandNames())
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
