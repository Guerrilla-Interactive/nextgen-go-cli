package screens

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// actionRow defines the dedicated Action Row commands.
var actionRow = []string{"undo", "redo", "paste from clipboard", "view project stats"}

// excluded commands for history/listing purposes
var excluded = map[string]bool{
	"undo":                     true,
	"redo":                     true,
	"show all my commands":     true, // Assuming this is a navigation command
	"view project stats":       true,
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
					// Handle actions (view stats, paste)
					if strings.ToLower(itemName) == "view project stats" {
						m.CurrentScreen = app.ScreenProjectStats
						return m, nil
					} else if strings.ToLower(itemName) == "paste from clipboard" {
						m.PendingCommand = itemName
						// Pass projectPath and registry to requiresMultipleVars
						if requiresMultipleVars(itemName, m.ProjectPath, registry) {
							// Set up for multi-variable prompt
							m.MultipleVariables = true
							m.VariableKeys = extractVariableKeys(itemName, m.ProjectPath, registry)
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
						m = UpdateFilenamePromptPreview(m, registry)
						return m, cursor.Blink
					}
					// TODO: Handle undo/redo if implemented
				}
			} else { // Focus is "list"
				if realIndex < totalCmds { // Ensure index is valid
					itemName := fullCommandList[realIndex]
					// Use HandleCommandSelection and capture both model and command
					var selectCmd tea.Cmd
					var updatedModel *app.Model // Temporary variable for the model pointer
					updatedModel, selectCmd = HandleCommandSelection(&m, registry, itemName)
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
	case "view project stats":
		// Use the new helper function to generate the preview
		m.StatsPreview = renderProjectInfoSection(m, registry)
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

	// --- Header & Project Info (Moved here to be part of left pane) ---
	// Spacer before logo
	verticalSpace := lipgloss.NewStyle().Height(3).Render("") // Reduced vertical space
	logo := app.TitleStyle.Render("NEXT") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("GEN") +
		" CLI" + lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(" "+m.Version) + "\n"
	projectStats := app.SummarizeProjectStats(m.RecognizedPkgs) // Limited stats for header
	headerSection := verticalSpace + logo + projectStats        // Combine header elements

	// --- Static Action Bar (Now with potential focus) ---
	actionBarText := renderStaticActionBar(actionRow, m.ActionIndex, m.MainScreenFocus == "action") // Pass index and focus

	// --- Left Pane: Paginated Command List ---
	var listBuilder strings.Builder

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
	// Calculate paginator view *before* defining left content
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
	rightPanelWidth := m.TerminalWidth - leftPanelWidth - 1 // Account for left panel width and space separator
	if rightPanelWidth < 30 {
		rightPanelWidth = 30
	} // Ensure minimum reasonable width
	rightPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Height(availableHeightForPanes). // Set explicit height
		Render(previewContentWithHeader)

	// --- Render Left Panel ---
	// Now render the left panel with the combined content and calculated height
	leftPanel := leftPanelStyle.
		Width(leftPanelWidth).
		Height(availableHeightForPanes). // Set explicit height to match right panel
		Render(leftContentCombined)

	// --- Combine Panes ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel) // Use single space separator

	// --- Footer ---
	// Render the final footer using the content without the paginator
	finalFooter := lipgloss.NewStyle().Render(footerContent)

	// --- Final Layout ---
	// HeaderSection is now part of combinedPanes (specifically leftPanel), so remove it from the final JoinVertical
	return lipgloss.JoinVertical(lipgloss.Left, combinedPanes, "\n", finalFooter) // Use finalFooter here
}

// renderStaticActionBar renders the action bar icons horizontally.
// It now accepts the selected index and focus state for highlighting.
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
		case "view project stats":
			icon = "📦"
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
func UpdateScreenProjectStats(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	numOptions := 6 // Path, Packages, Usage, History, Manage Commands, Back (Removed Project Commands)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.StatsScreenIndex = (m.StatsScreenIndex + numOptions - 1) % numOptions

	case "down", "j":
		m.StatsScreenIndex = (m.StatsScreenIndex + 1) % numOptions

	case "enter":
		switch m.StatsScreenIndex {
		case 0: // Project Info
		case 1: // Detected Packages
		case 2: // Project Usage
			// No action yet
		case 3: // Command History
			m.CurrentScreen = app.ScreenCommandHistory
			m.HistoryScreenIndex = 0
			m = updateHistoryPreview(m, registry)
			return m, nil
		case 4: // Manage Commands (Previously Project Commands)
			m.CurrentScreen = app.ScreenCommandsCategory
			m.CommandsCategoryIndex = 0
			return m, nil
		case 5: // Back (Previously index 6)
			m.CurrentScreen = app.ScreenMain
			m.StatsScreenIndex = 0
			return m, nil
		}

	case "esc", "b": // Go back to Main
		m.CurrentScreen = app.ScreenMain
		m.StatsScreenIndex = 0
		return m, nil
	}

	return m, nil
}

// ViewProjectStatsScreen renders the full project stats (all recognized packages)
// along with a header and footer.
// For the full version that includes project registry data, use ViewProjectStatsScreenWithRegistry.
func ViewProjectStatsScreen(m app.Model) string {
	// This is a backward-compatible wrapper for apps that don't yet have access to the registry
	return ViewProjectStatsScreenWithRegistry(m, nil)
}

// ViewProjectStatsScreenWithRegistry renders the interactive project stats screen.
func ViewProjectStatsScreenWithRegistry(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Project Stats") + "\n"

	// --- Left Pane: Navigation ---
	navItems := []string{"Project Info", "Detected Packages", "Project Usage", "Command History", "Manage Commands", "Back"} // Removed Project Commands
	var leftBuilder strings.Builder
	leftBuilder.WriteString(app.SubtitleStyle.Render("Categories") + "\n\n")
	for i, item := range navItems {
		if i == m.StatsScreenIndex {
			leftBuilder.WriteString(app.HighlightStyle.Render("> "+item) + "\n")
		} else {
			leftBuilder.WriteString(app.ChoiceStyle.Render("  "+item) + "\n")
		}
	}
	// Use a fixed width for the left panel for consistent layout
	leftPanel := lipgloss.NewStyle().
		Width(50).
		Padding(2, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Render(leftBuilder.String())

	// --- Right Pane: Details Preview ---
	var previewContent string
	switch m.StatsScreenIndex {
	case 0: // Project Info (Path, Type)
		var pb strings.Builder
		pb.WriteString(app.SubtitleStyle.Render("Project Info") + "\n\n")
		if m.ProjectPath != "" {
			pb.WriteString("Path: " + app.PathStyle.Render(m.ProjectPath) + "\n")
			if registry != nil {
				if info, found := registry.GetProject(m.ProjectPath); found && info.Type != "" {
					pb.WriteString(fmt.Sprintf("Type: %s\n", info.Type))
				}
			}
		} else {
			pb.WriteString(app.ChoiceStyle.Render("Path not available.") + "\n")
		}
		previewContent = pb.String()
	case 1: // Detected Packages
		previewContent = app.SubtitleStyle.Render("Detected Packages") + "\n\n"
		if len(m.RecognizedPkgs) > 0 {
			previewContent += app.SummarizeFullProjectStats(m.RecognizedPkgs) // Uses the existing summarization
		} else {
			previewContent += app.ChoiceStyle.Render("No packages detected.")
		}
	case 2: // Project Usage (Count, Last Access)
		var pb strings.Builder
		pb.WriteString(app.SubtitleStyle.Render("Project Usage") + "\n\n")
		if registry != nil && m.ProjectPath != "" {
			if info, found := registry.GetProject(m.ProjectPath); found {
				pb.WriteString(fmt.Sprintf("- Count: %d\n", info.UsageCount))
				lastAccess := time.Unix(info.LastAccessTime, 0)
				pb.WriteString(fmt.Sprintf("- Last Access: %s\n", lastAccess.Format("Jan 2, 2006 at 3:04 PM")))
			} else {
				pb.WriteString(app.ChoiceStyle.Render("  (Project usage not recorded yet)\n"))
			}
		} else {
			pb.WriteString(app.ChoiceStyle.Render("  (Registry or Project Path not available)\n"))
		}
		previewContent = pb.String()
	case 3: // Command History
		var pb strings.Builder
		pb.WriteString(app.SubtitleStyle.Render("Recent Commands (Preview)") + "\n\n") // Update title
		if registry != nil && m.ProjectPath != "" {
			if info, found := registry.GetProject(m.ProjectPath); found && len(info.CommandHistory) > 0 {
				// Display only the names of the last N commands
				maxToShow := 10 // Or adjust as needed for preview space
				start := 0
				if len(info.CommandHistory) > maxToShow {
					start = len(info.CommandHistory) - maxToShow
				}
				for i := start; i < len(info.CommandHistory); i++ {
					// Display command name with a simple list format
					pb.WriteString(fmt.Sprintf("- %s\n", info.CommandHistory[i].Name))
				}
			} else {
				pb.WriteString(app.ChoiceStyle.Render("  (No commands recorded yet)\n"))
			}
		} else {
			pb.WriteString(app.ChoiceStyle.Render("  (History not available)\n"))
		}
		previewContent = pb.String()
	case 4: // Manage Commands Preview (Previously Project Commands Preview)
		previewContent = app.SubtitleStyle.Render("Recent Clipboard Commands") + "\n\n"
		if registry != nil && len(registry.ClipboardCommands) > 0 {
			// Get clipboard commands and sort by timestamp descending
			cmds := make([]project.ClipboardCommandSpec, 0, len(registry.ClipboardCommands))
			for _, spec := range registry.ClipboardCommands {
				cmds = append(cmds, spec)
			}
			sort.SliceStable(cmds, func(i, j int) bool {
				return cmds[i].Timestamp > cmds[j].Timestamp // Newest first
			})

			// --- Limit display ---
			limit := 7
			displayedCount := 0
			for _, cmd := range cmds {
				if displayedCount >= limit {
					previewContent += app.ChoiceStyle.Render("  ...") + "\n"
					break
				}
				previewContent += fmt.Sprintf("- %s\n", cmd.Name)
				displayedCount++
			}
		} else {
			previewContent += app.ChoiceStyle.Render("  (No clipboard commands saved yet)\n")
		}
	case 5: // Back (Previously index 6)
		previewContent = app.HelpStyle.Render("Select an item on the left to view details.")
	default:
		previewContent = "Unknown selection."
	}

	// Apply common styling to the right panel
	rightPanel := lipgloss.NewStyle().
		Padding(0, 0).
		Height(lipgloss.Height(leftPanel)). // Match height roughly
		Border(lipgloss.RoundedBorder()).
		Render(previewContent)

	// --- Combine Panes ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// --- Footer ---
	footer := app.HelpStyle.Render("Use ↑/↓ to navigate, Enter on Back (or Esc/b) to return.")

	// --- Final Layout ---
	return lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", footer)
}

// Helper function to get the full prioritized list of commands for the main screen
func getPrioritizedCommandList(m *app.Model, registry *project.ProjectRegistry) []string {
	// Use a map to track added commands and prevent duplicates
	added := make(map[string]bool)
	var fullList []string

	// 1. Recent Commands (Top 5, excluding actions)
	recentLimit := 5
	count := 0
	for _, cmd := range commands.RecentUsed {
		lower := strings.ToLower(cmd)
		if excluded[lower] {
			continue
		}
		if !added[cmd] && count < recentLimit {
			fullList = append(fullList, cmd)
			added[cmd] = true
			count++
		}
	}

	// 2. Favorite Native Commands
	if registry != nil && registry.FavoriteNativeCommands != nil {
		var favNative []string
		for cmdName := range registry.FavoriteNativeCommands {
			favNative = append(favNative, cmdName)
		}
		sort.Strings(favNative) // Sort favorites alphabetically
		for _, cmd := range favNative {
			if !added[cmd] {
				fullList = append(fullList, cmd)
				added[cmd] = true
			}
		}
	}

	// 3. Favorite Clipboard Commands
	if registry != nil && registry.ClipboardCommands != nil {
		var favClipboard []project.ClipboardCommandSpec
		for _, spec := range registry.ClipboardCommands {
			if spec.IsFavorite {
				favClipboard = append(favClipboard, spec)
			}
		}
		// Sort favorites by timestamp, newest first
		sort.SliceStable(favClipboard, func(i, j int) bool {
			return favClipboard[i].Timestamp > favClipboard[j].Timestamp
		})
		for _, spec := range favClipboard {
			if !added[spec.Name] {
				fullList = append(fullList, spec.Name)
				added[spec.Name] = true
			}
		}
	}

	// 4. Local Project Commands
	localCmds, _ := getSortedProjectCommandNames(m.ProjectPath) // Ignore error here
	for _, cmd := range localCmds {
		if !added[cmd] {
			fullList = append(fullList, cmd)
			added[cmd] = true
		}
	}

	// 5. Remaining Native Commands
	allNative := commands.AllCommandNames()
	sort.Strings(allNative) // Sort alphabetically
	for _, cmd := range allNative {
		if !added[cmd] && !excluded[strings.ToLower(cmd)] {
			fullList = append(fullList, cmd)
			added[cmd] = true
		}
	}

	// 6. Remaining Clipboard Commands (non-favorite)
	if registry != nil && registry.ClipboardCommands != nil {
		var otherClipboard []string
		for name, spec := range registry.ClipboardCommands {
			if !spec.IsFavorite {
				otherClipboard = append(otherClipboard, name)
			}
		}
		sort.Strings(otherClipboard)
		for _, cmd := range otherClipboard {
			if !added[cmd] {
				fullList = append(fullList, cmd)
				added[cmd] = true
			}
		}
	}

	return fullList
}
