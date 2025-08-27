package mainScreen

import (
	"fmt"
	"os"
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

// excluded commands for history/listing purposes
var excluded = map[string]bool{
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
				if strings.ToLower(actionRow[m.ActionIndex]) == "paste from clipboard" {
					// Force refresh preview for clipboard
					m = updatePreview(m, registry, actionRow[m.ActionIndex])
				}
			}

		case "right", "l":
			// Only paginate if list has focus and not on last page
			if m.MainScreenFocus == "list" && totalCmds > 0 && !p.OnLastPage() {
				*p, paginatorCmd = p.Update(keyMsg) // Update paginator here
			} else if m.MainScreenFocus == "action" {
				// Move focus within action bar
				m.ActionIndex = (m.ActionIndex + 1) % len(actionRow)
				m = updatePreview(m, registry, actionRow[m.ActionIndex])
				if strings.ToLower(actionRow[m.ActionIndex]) == "paste from clipboard" {
					// Force refresh preview for clipboard
					m = updatePreview(m, registry, actionRow[m.ActionIndex])
				}
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
					if strings.ToLower(actionRow[m.ActionIndex]) == "paste from clipboard" {
						m = updatePreview(m, registry, actionRow[m.ActionIndex])
					}
				} else if totalCmds > 0 {
					m.SelectedIndex--
				}
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
						// Determine variables; if none, run directly without prompt
						keys, _ := commands.ExtractVariablesFromClipboard()
						if len(keys) == 0 {
							m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", itemName)
							m.CurrentScreen = app.ScreenInstallDetails
							return m, commands.RunCommand(itemName, m.ProjectPath, nil, registry)
						}
						// Otherwise, go to prompt
						m.MultipleVariables = len(keys) > 1
						m.VariableKeys = keys
						m.CurrentVariableIndex = 0
						m.Variables = make(map[string]string)
						m.CurrentScreen = app.ScreenFilenamePrompt
						m.TempFilename = ""
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
					// Combine selection with paginator command (nil-safe in Batch)
					return m, tea.Batch(paginatorCmd, selectCmd)
				}
			}
		}
	}

	// --- Handle Clipboard Refresh Tick ---
	// Removed per-screen clipboard tick

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
	case "paste from clipboard":
		// Generate preview based on clipboard content
		// Use default placeholders as we don't have real input yet
		placeholderMap := commands.BuildAutoPlaceholders(map[string]string{"Filename": "<PastedItem>"})
		pv, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath)
		if err == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		} else {
			// Clean, minimal guidance panel using Lipgloss
			muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
			title := app.SubtitleStyle.Render("Clipboard Preview")
			msg := muted.Render("Copy a NextGen JSON command, then choose ‘paste from clipboard’.")
			content := lipgloss.JoinVertical(lipgloss.Left, title, "", msg)
			panel := lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Render(content)
			m.FileTreePreview = panel
			m.CurrentPreviewType = "file-tree"
		}
	default:
		// Attempt to generate file tree preview for other commands
		// First, handle saved clipboard commands using their stored template
		if registry != nil {
			if cmdSpec, ok := registry.ClipboardCommands[selectedCmdName]; ok {
				placeholderMap := commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
				pv, err := commands.GeneratePreviewFileTreeFromBytes([]byte(cmdSpec.Template), placeholderMap, m.ProjectPath)
				if err == nil && strings.TrimSpace(pv) != "" {
					m.FileTreePreview = pv
					m.CurrentPreviewType = "file-tree"
					return m
				}
				// If clipboard template preview fails, fall through to generic handling below
			}
		}

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

		// Next, handle project-local commands (.nextgen/local-commands)
		if m.ProjectPath != "" {
			kebab := commands.ToKebabCase(selectedCmdName)
			localPath := filepath.Join(m.ProjectPath, ".nextgen", "local-commands", kebab+".json")
			if data, readErr := os.ReadFile(localPath); readErr == nil {
				if pv, perr := commands.GeneratePreviewFileTreeFromBytes(data, placeholderMap, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
					m.FileTreePreview = pv
					m.CurrentPreviewType = "file-tree"
					return m
				}
			}
		}

		// Pass the potentially updated selectedCmdName if it was clipboard paste
		pv, err2 := commands.GeneratePreviewFileTree(selectedCmdName, placeholderMap, m.ProjectPath)
		if err2 == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		} else {
			m.CurrentPreviewType = "none"
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
	leftPanelWidth := 50 // Keep left panel fixed width
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
		// previewContent += "\n... (truncated)"
	}

	// --- Prepend header and render final right panel ---
	folderName := filepath.Base(m.ProjectPath)
	previewHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContentWithHeader := previewHeader + "\n\n" + previewContent

	// Build right panel content and bottom-align within available height
	rightInner := lipgloss.NewStyle().Padding(1, 1).Render(previewContentWithHeader)
	rightPanel := lipgloss.Place(lipgloss.Width(rightInner), availableHeightForPanes, lipgloss.Left, lipgloss.Bottom, rightInner)

	// --- Render Left Panel ---

	leftRendered := leftPanelStyle.Render(leftContentCombined)
	leftPanel := lipgloss.Place(leftPanelWidth, availableHeightForPanes, lipgloss.Left, lipgloss.Bottom, leftRendered)

	// --- Combine Panes ---
	// Lipgloss JoinHorizontal will distribute remaining space to the right panel
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)

	// --- Footer ---
	// Render the final footer using the content without the paginator
	finalFooter := lipgloss.NewStyle().Render(footerContent)

	// --- Final Layout ---
	finalLayout := lipgloss.JoinVertical(lipgloss.Left, combinedPanes, "\n", finalFooter)
	// Align the entire screen to the bottom of the terminal if dimensions are known
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalLayout)
	}
	return finalLayout
}

// actionRow defines the dedicated Action Row commands.
var actionRow = []string{"paste from clipboard", "View Settings"}

// renderStaticActionBar creates the interactive action bar.
func renderStaticActionBar(items []string, selectedIndex int, hasFocus bool) string {
	var actionBarItems []string
	for i, val := range items {
		lowerVal := strings.ToLower(val)
		icon := "?"
		if lowerVal == "paste from clipboard" {
			icon = "Paste"
		} else if lowerVal == "view settings" {
			icon = "Settings"
		}
		// Apply highlight if this item has focus
		itemText := icon // Default text is just the icon
		itemStyle := app.ChoiceStyle
		if hasFocus && i == selectedIndex {
			itemStyle = app.HighlightStyle.Copy().Bold(true) // Keep highlight style, maybe remove Reverse?
			itemText = fmt.Sprintf("> %s <", icon)           // Add markers
		}
		actionBarItems = append(actionBarItems, lipgloss.NewStyle().Padding(0, 3, 0, 0).Render(itemStyle.Render(itemText)))
	}
	// Join horizontally and remove border styling
	return lipgloss.NewStyle().
		MarginBottom(1).
		Padding(0, 0).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, actionBarItems...))
}

// getPrioritizedCommandList retrieves and orders the list of commands for the main screen.
func getPrioritizedCommandList(m *app.Model, registry *project.ProjectRegistry) []string {
	const maxRecent = 4 // Show top 4 recent commands

	// Maps to track added commands and avoid duplicates
	added := make(map[string]bool)
	resultList := []string{}

	// --- 1. Top 4 Recent Commands ---
	recentCount := 0
	for _, cmd := range commands.RecentUsed {
		if recentCount >= maxRecent {
			break
		}
		lower := strings.ToLower(cmd)
		if excluded[lower] { // Skip excluded actions like settings, paste, etc.
			continue
		}
		if !added[cmd] {
			resultList = append(resultList, cmd)
			added[cmd] = true
			recentCount++
		}
	}

	// --- Prepare lists for remaining commands ---
	var allFavorites []string
	var remainingNative []string
	var remainingOthers []string // Clipboard + Project

	// --- Categorize Clipboard Commands ---
	if registry != nil && registry.ClipboardCommands != nil {
		for name, spec := range registry.ClipboardCommands {
			if !added[name] && !excluded[strings.ToLower(name)] {
				if spec.IsFavorite {
					allFavorites = append(allFavorites, name)
				} else {
					remainingOthers = append(remainingOthers, name)
				}
			}
		}
	}

	// --- Categorize Native Commands ---
	nativeNames := commands.AllCommandNames()
	for _, name := range nativeNames {
		if !added[name] && !excluded[strings.ToLower(name)] {
			if isFav, ok := registry.FavoriteNativeCommands[name]; ok && isFav {
				allFavorites = append(allFavorites, name)
			} else {
				remainingNative = append(remainingNative, name)
			}
		}
	}

	// --- Categorize Project Commands ---
	if registry != nil && m.ProjectPath != "" {
		projectCmdNames, err := projectCmdScreen.GetSortedProjectCommandNames(m.ProjectPath)
		if err == nil {
			for _, name := range projectCmdNames {
				if !added[name] && !excluded[strings.ToLower(name)] {
					if isFav, ok := registry.FavoriteProjectCommands[name]; ok && isFav {
						allFavorites = append(allFavorites, name)
					} else {
						remainingOthers = append(remainingOthers, name)
					}
				}
			}
		}
	}

	// --- Sort the categorized lists ---
	sort.Strings(allFavorites)
	sort.Strings(remainingNative)
	sort.Strings(remainingOthers)

	// --- Append to the result list in the desired order ---
	// Helper to append unique items
	appendUnique := func(target *[]string, source []string) {
		for _, cmd := range source {
			if !added[cmd] {
				*target = append(*target, cmd)
				added[cmd] = true // Mark as added to final list
			}
		}
	}

	appendUnique(&resultList, allFavorites)
	appendUnique(&resultList, remainingNative)
	appendUnique(&resultList, remainingOthers)

	return resultList
}
