package mainScreen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	projectCmdScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actionRow defines the dedicated Action Row commands.
var actionRow = []string{"undo", "redo", "paste from clipboard", "View Settings"}

// excluded commands for history/listing purposes
var excluded = map[string]bool{
	"undo":                     true,
	"redo":                     true,
	"show all my commands":     true, // Assuming this is a navigation command
	"view settings":            true, // Renamed
	"logoutorloginplaceholder": true, // Assuming this is navigation/action
	"paste from clipboard":     true, // Special handling, not listed directly
}

// UpdateScreenMain handles input for the main screen, now using pagination and focus.
func UpdateScreenMain(m app.Model, msg tea.Msg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {

	// Get the full ordered list of commands
	fullCommandList := getPrioritizedCommandList(&m, registry)
	totalCmds := len(fullCommandList)

	// --- Paginator Setup ---
	m.MainListPaginator.SetTotalPages(totalCmds)
	p := &m.MainListPaginator
	var paginatorCmd tea.Cmd // Store paginator command result here
	// DONT update paginator here: *p, paginatorCmd = p.Update(msg)

	// --- Calculate index and page options ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	// Clamp list index (m.SelectedIndex) to the number of items on the current page
	if m.SelectedIndex >= numItemsOnPage {
		m.SelectedIndex = numItemsOnPage - 1
		if m.SelectedIndex < 0 {
			m.SelectedIndex = 0
		} // Ensure not negative if page empty
	}
	realIndex := start + m.SelectedIndex // Index in the full list

	// --- Handle Keypresses ---
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "left", "h":
			// Only paginate if list has focus and not on first page
			if m.MainScreenFocus == "list" && totalCmds > 0 && p.Page > 0 {
				*p, paginatorCmd = p.Update(keyMsg) // Update paginator here
			} else if m.MainScreenFocus == "action" {
				// Move focus within action bar
				m.ActionIndex = (m.ActionIndex + len(actionRow) - 1) % len(actionRow)
				m = updatePreview(m, registry, actionRow[m.ActionIndex])
			}

		case "right", "l":
			// Only paginate if list has focus and not on last page
			if m.MainScreenFocus == "list" && totalCmds > 0 && !p.OnLastPage() {
				*p, paginatorCmd = p.Update(keyMsg) // Update paginator here
			} else if m.MainScreenFocus == "action" {
				// Move focus within action bar
				m.ActionIndex = (m.ActionIndex + 1) % len(actionRow)
				m = updatePreview(m, registry, actionRow[m.ActionIndex])
			}

		case "up", "k":
			if m.MainScreenFocus == "list" {
				if m.SelectedIndex == 0 { // At top of list
					m.MainScreenFocus = "action" // Move focus to action bar
					// Optional: Try to select action item closest horizontally?
					// For simplicity, just focus the last remembered one or the first one.
					if m.LastActionIndex < 0 || m.LastActionIndex >= len(actionRow) {
						m.ActionIndex = 0
					} else {
						m.ActionIndex = m.LastActionIndex
					}
				} else if totalCmds > 0 {
					m.SelectedIndex--
				}
			} else { // Focus is "action"
				// Pressing up on action bar wraps around (handled by left/right for now)
			}

		case "down", "j":
			if m.MainScreenFocus == "action" {
				m.MainScreenFocus = "list"        // Move focus to list
				m.LastActionIndex = m.ActionIndex // Remember last action index
				m.SelectedIndex = 0               // Start at top of list
			} else if m.MainScreenFocus == "list" && totalCmds > 0 { // Focus is "list"
				// Navigate down the list on the current page
				if m.SelectedIndex < numItemsOnPage-1 {
					m.SelectedIndex++
				} else {
					// Optional: Wrap around to top? Or stop?
					// For now, let's stop at the bottom of the page.
				}
			}

		case "enter":
			if m.MainScreenFocus == "action" {
				if m.ActionIndex >= 0 && m.ActionIndex < len(actionRow) {
					itemName := actionRow[m.ActionIndex]
					// Handle actions (view settings, paste)
					if strings.ToLower(itemName) == "view settings" { // Renamed check
						m.CurrentScreen = app.ScreenSettings // Navigate to new screen
						return m, nil
					} else if strings.ToLower(itemName) == "paste from clipboard" {
						m.PendingCommand = itemName
						// Pass projectPath and registry to sharedScreens.RequiresMultipleVars
						if sharedScreens.RequiresMultipleVars(itemName, m.ProjectPath, registry) {
							// Set up for multi-variable prompt
							m.MultipleVariables = true
							m.VariableKeys = sharedScreens.ExtractVariableKeys(itemName, m.ProjectPath, registry)
							m.CurrentVariableIndex = 0
							m.Variables = make(map[string]string)
						} else {
							// Set up for single-variable prompt (Filename)
							m.MultipleVariables = false
							m.VariableKeys = []string{"Filename"} // Default for clipboard paste
						}
						m.CurrentScreen = app.ScreenFilenamePrompt
						m.TempFilename = ""
						// Update preview for prompt screen
						return m, cursor.Blink
					}
					// TODO: Handle undo/redo if implemented
				}
			} else { // Focus is "list"
				if realIndex < totalCmds { // Ensure index is valid
					itemName := fullCommandList[realIndex]
					// Use sharedScreens.HandleCommandSelection and capture both model and command
					var selectCmd tea.Cmd
					var updatedModel *app.Model // Temporary variable for the model pointer
					updatedModel, selectCmd = sharedScreens.HandleCommandSelection(&m, registry, itemName)
					m = *updatedModel             // Assign the dereferenced model back to m
					m.CurrentPreviewType = "none" // Clear preview after selection
					m.FileTreePreview = ""
					m.StatsPreview = ""
					// We need to combine the selection command with any paginator command
					// Check if paginatorCmd is nil before batching
					if paginatorCmd != nil {
						return m, tea.Batch(paginatorCmd, selectCmd)
					} else {
						return m, selectCmd
					}
				}
			}
		}
	}

	// --- Update Preview (moved down to ensure paginatorCmd is potentially set) ---
	// Check if paginator needs update even if no key press triggered it (e.g. WindowSizeMsg)
	// We only update the paginator model variable, the actual cmd is returned at the end.

	// We need to recalculate realIndex AFTER potential paginator updates
	start, _ = p.GetSliceBounds(totalCmds) // Recalculate start index
	realIndex = start + m.SelectedIndex    // Recalculate real index

	if m.MainScreenFocus == "list" && realIndex >= 0 && realIndex < totalCmds {
		m = updatePreview(m, registry, fullCommandList[realIndex])
	} else if m.MainScreenFocus == "action" && m.ActionIndex >= 0 && m.ActionIndex < len(actionRow) {
		m = updatePreview(m, registry, actionRow[m.ActionIndex])
	} else {
		// Clear preview if nothing relevant is focused
		m.CurrentPreviewType = "none"
		m.FileTreePreview = ""
		m.StatsPreview = ""
	}

	// Return the updated model and any command from the paginator or other logic
	return m, paginatorCmd
}

// updatePreview needs modification to accept the selected command name directly
func updatePreview(m app.Model, registry *project.ProjectRegistry, selectedCmdName string) app.Model {
	lowerCmd := strings.ToLower(selectedCmdName)

	// Reset previews
	m.FileTreePreview = ""
	m.StatsPreview = ""
	m.CurrentPreviewType = "none"

	switch lowerCmd {
	case "view settings": // Renamed
		// Use sharedScreens.RenderProjectInfoSection
		m.StatsPreview = sharedScreens.RenderProjectInfoSection(m, registry)
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
		// Use the new function to get keys
		keys, err := commands.GetCommandVariableKeys(selectedCmdName, m.ProjectPath, registry)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			// Use the first key as the primary placeholder for preview
			placeholders := map[string]string{keys[0]: "<" + keys[0] + ">"}
			placeholderMap = commands.BuildPlaceholders(placeholders)
		} else {
			// Fallback if no keys found or error getting keys
			placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Filename>"})
		}

		// Pass the potentially updated selectedCmdName if it was clipboard paste
		pv, err2 := commands.GeneratePreviewFileTree(selectedCmdName, placeholderMap, m.ProjectPath)
		if err2 == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		} else {
			m.CurrentPreviewType = "none"
			if err2 != nil {
				// Optional: Log error: fmt.Printf("Preview error for %s: %v\n", selectedCmdName, err2)
			}
		}
	}
	return m
}

// ViewMainScreen is the view for the main screen.
func ViewMainScreen(m app.Model, registry *project.ProjectRegistry) string {
	// --- Get Data ---
	fullCommandList := getPrioritizedCommandList(&m, registry)
	totalCmds := len(fullCommandList)
	p := m.MainListPaginator
	start, end := p.GetSliceBounds(totalCmds)
	paginatedCmds := []string{}
	if start < end {
		paginatedCmds = fullCommandList[start:end]
	}

	// --- Header (Removed project stats summary) ---
	verticalSpace := lipgloss.NewStyle().Height(3).Render("") // Reduced vertical space
	logo := app.TitleStyle.Render("NEXT") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("GEN") +
		" CLI" + lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(" "+m.Version) + "\n"
	// projectStats := app.SummarizeProjectStats(m.RecognizedPkgs) // REMOVED Limited stats for header
	headerSection := verticalSpace + logo // Combine header elements (without stats)

	// --- Static Action Bar (Now with potential focus) ---
	actionBarText := renderStaticActionBar(actionRow, m.ActionIndex, m.MainScreenFocus == "action") // Pass index and focus

	// --- Left Pane: Paginated Command List ---
	var listBuilder strings.Builder

	// Ensure paginator knows total pages before rendering its view
	p.SetTotalPages(totalCmds)

	if totalCmds == 0 {
		listBuilder.WriteString(" (No commands available)")
	} else {
		for i, cmdName := range paginatedCmds {
			// Check favorite status
			prefix := ""
			if registry != nil {
				if isFav, ok := registry.FavoriteNativeCommands[cmdName]; ok && isFav {
					prefix = "⭐ "
				} else if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok && cmdSpec.IsFavorite {
					prefix = "⭐ "
				} else if isFav, ok := registry.FavoriteProjectCommands[cmdName]; ok && isFav {
					prefix = "⭐ "
				}
			}

			// Only highlight if list has focus
			if m.MainScreenFocus == "list" && i == m.SelectedIndex {
				listBuilder.WriteString(app.HighlightStyle.Render(prefix+cmdName) + "\n")
			} else {
				listBuilder.WriteString(app.ChoiceStyle.Render(prefix+cmdName) + "\n")
			}
		}
	}
	// --- Combine Left Pane Content (Header + Action Bar + List) ---
	// Calculate paginator view *after* ensuring total pages is set
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}
	leftContentCombined := lipgloss.JoinVertical(lipgloss.Left, headerSection, actionBarText, listBuilder.String(), paginatorView) // Added paginatorView here
	// Apply NO border or fixed width to the combined left content itself
	leftPanelStyle := lipgloss.NewStyle().Padding(0, 1) // Just padding
	// Calculate width dynamically based on longest item? For now, maybe fixed is ok?
	// Let's try a slightly smaller fixed width for commands again.
	leftPanelWidth := 40 // Keep it smaller
	// Left panel rendering moved after height calculation

	// --- Define Footer Content First (Without Paginator) ---
	var statusLine string
	if m.HistorySaveStatus != "" {
		// ... status line styling ...
		if strings.HasPrefix(m.HistorySaveStatus, "Error:") {
			statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.HistorySaveStatus)
		} else {
			statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.HistorySaveStatus)
		}
		statusLine += "\n"
	}
	footerHelp := app.HelpStyle.Render("Use ↑/↓/←/→ to navigate, Enter to select, q quits.")
	footerContent := statusLine + footerHelp // Paginator is removed from here

	// --- Calculate available height for panels ---
	footerHeight := lipgloss.Height(footerContent) // Calculate height based on new footer content
	// Calculate available height for the row containing left/right panes
	// Subtract footer height and the newline separator between panes and footer
	availableHeightForPanes := m.TerminalHeight - footerHeight - 1
	if availableHeightForPanes < 10 { // Ensure minimum height for usability
		availableHeightForPanes = 10
	}

	// --- Right Pane: Preview ---
	var previewContent string
	// Determine the raw preview content based on type
	switch m.CurrentPreviewType {
	case "stats":
		// For now, if stats preview is somehow set, display it, but it shouldn't be set by updatePreview.
		previewContent = m.StatsPreview
	case "file-tree":
		previewContent = m.FileTreePreview
	default:
		previewContent = "No preview available for this command."
	}

	// --- Truncate raw content BEFORE adding header/styling ---
	// Account for the header lines and padding added to the preview content
	folderHeaderHeight := 2 // "📦 folderName\n\n"
	previewPadding := 2     // Top/bottom padding of 1 each
	maxPreviewContentHeight := availableHeightForPanes - folderHeaderHeight - previewPadding
	if maxPreviewContentHeight < 1 {
		maxPreviewContentHeight = 1
	}
	lines := strings.Split(previewContent, "\n")
	if len(lines) > maxPreviewContentHeight {
		previewContent = strings.Join(lines[:maxPreviewContentHeight], "\n")
		previewContent += "\n... (truncated)"
	}

	// --- Prepend header and render final right panel ---
	folderName := filepath.Base(m.ProjectPath)
	previewHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContentWithHeader := previewHeader + "\n\n" + previewContent
	// Render with consistent padding, no border, and explicit height
	rightPanelWidth := m.TerminalWidth - leftPanelWidth - 1 // Adjust width based on terminal size
	if rightPanelWidth < 10 {
		rightPanelWidth = 10
	}
	rightPanelStyle := lipgloss.NewStyle().
		Padding(1, 1).                   // Consistent padding
		Height(availableHeightForPanes). // Set explicit height
		Width(rightPanelWidth)           // Set explicit width
	rightPanel := rightPanelStyle.Render(previewContentWithHeader)

	// --- Render Left Panel ---
	// Now render the left panel with the combined content and calculated height
	leftPanel := leftPanelStyle.
		Width(leftPanelWidth).
		Height(availableHeightForPanes). // Set explicit height to match right panel
		Render(leftContentCombined)

	// --- Combine Panes ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel) // Use minimal space between

	// --- Footer ---
	// Render the final footer using the content without the paginator
	finalFooter := lipgloss.NewStyle().Render(footerContent)

	// --- Final Layout ---
	// HeaderSection is now part of combinedPanes (specifically leftPanel), so remove it from the final JoinVertical
	return lipgloss.JoinVertical(lipgloss.Left, combinedPanes, "\n", finalFooter) // Use finalFooter here
}

// renderStaticActionBar creates the interactive action bar.
func renderStaticActionBar(items []string, selectedIndex int, hasFocus bool) string {
	var actionBarItems []string
	for i, val := range items {
		lowerVal := strings.ToLower(val)
		var icon string
		switch lowerVal {
		case "undo":
			icon = "↺"
		case "redo":
			icon = "↻"
		case "paste from clipboard":
			icon = "📋"
		case "view settings": // Renamed
			icon = "⚙️" // Changed icon
		default:
			icon = "?"
		}
		// Apply highlight if this item has focus
		itemText := icon // Default text is just the icon
		itemStyle := app.ChoiceStyle
		if hasFocus && i == selectedIndex {
			itemStyle = app.HighlightStyle.Copy().Bold(true) // Keep highlight style, maybe remove Reverse?
			itemText = fmt.Sprintf("> %s <", icon)           // Add markers
		}
		actionBarItems = append(actionBarItems, lipgloss.NewStyle().Padding(0, 1).Render(itemStyle.Render(itemText)))
	}
	// Join horizontally and remove border styling
	return lipgloss.NewStyle().
		MarginBottom(1).
		Padding(0, 0).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, actionBarItems...))
}

// UpdateScreenProjectStats handles input on the Project Stats screen.
// MOVED to settings.screen.go

// ViewProjectStatsScreen renders the full project stats (all recognized packages)
// along with a header and footer.
// MOVED to settings.screen.go

// ViewProjectStatsScreenWithRegistry renders the interactive project stats screen.
// MOVED to settings.screen.go

// getPrioritizedCommandList retrieves and orders the list of commands for the main screen.
func getPrioritizedCommandList(m *app.Model, registry *project.ProjectRegistry) []string {
	// Start with recently used (maintained in-memory for the session)
	recent := commands.RecentUsed

	// Prepare maps for quick lookup
	recentMap := make(map[string]bool)
	for _, cmd := range recent {
		recentMap[cmd] = true
	}
	excludedMap := excluded // Use the package-level excluded map

	// Combine other command sources, excluding recent and explicitly excluded ones
	otherCmds := []string{}

	// Add Clipboard Commands (Sorted)
	if registry != nil && registry.ClipboardCommands != nil {
		clipboardNames := make([]string, 0, len(registry.ClipboardCommands))
		for name := range registry.ClipboardCommands {
			clipboardNames = append(clipboardNames, name)
		}
		sort.Strings(clipboardNames)
		for _, name := range clipboardNames {
			if !recentMap[name] && !excludedMap[strings.ToLower(name)] {
				otherCmds = append(otherCmds, name)
			}
		}
	}

	// Add Native Commands (Sorted)
	nativeNames := commands.AllCommandNames() // Assuming this returns sorted names
	for _, name := range nativeNames {
		if !recentMap[name] && !excludedMap[strings.ToLower(name)] {
			otherCmds = append(otherCmds, name)
		}
	}

	// Add Project Commands (Sorted)
	if registry != nil && m.ProjectPath != "" {
		projectCmdNames, err := projectCmdScreen.GetSortedProjectCommandNames(m.ProjectPath)
		if err == nil {
			for _, name := range projectCmdNames {
				if !recentMap[name] && !excludedMap[strings.ToLower(name)] {
					otherCmds = append(otherCmds, name)
				}
			}
		}
	}

	// --- Sort other commands by priority (Favorites first) ---
	sort.SliceStable(otherCmds, func(i, j int) bool {
		nameI := otherCmds[i]
		nameJ := otherCmds[j]
		isFavI := isFavorite(nameI, registry)
		isFavJ := isFavorite(nameJ, registry)

		if isFavI && !isFavJ {
			return true // Favorites come first
		}
		if !isFavI && isFavJ {
			return false
		}
		// If both are favorite or both not, maintain alphabetical order (already sorted by type)
		return nameI < nameJ // Fallback to alphabetical if same favorite status
	})

	// Combine recent (already ordered) with sorted others
	fullList := append(recent, otherCmds...)

	return fullList
}

// isFavorite checks if a command is marked as favorite in any category.
func isFavorite(cmdName string, registry *project.ProjectRegistry) bool {
	if registry == nil {
		return false
	}
	if isFav, ok := registry.FavoriteNativeCommands[cmdName]; ok && isFav {
		return true
	}
	if cmdSpec, ok := registry.ClipboardCommands[cmdName]; ok && cmdSpec.IsFavorite {
		return true
	}
	if isFav, ok := registry.FavoriteProjectCommands[cmdName]; ok && isFav {
		return true
	}
	return false
}
