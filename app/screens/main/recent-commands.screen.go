package mainScreen

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	projectCmdScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	config "github.com/Guerrilla-Interactive/nextgen-go-cli/internal"
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

// truncateWithEllipsis limits s to max runes and appends … if truncated.
func truncateWithEllipsis(s string, max int) string {
	r := []rune(s)
	if max <= 0 {
		return ""
	}
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// relativeTimeShort returns a short relative time string without "ago".
func relativeTimeShort(ts int64) string {
	if ts == 0 {
		return "now"
	}
	d := time.Since(time.Unix(ts, 0))
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		s := int(d.Seconds())
		if s <= 0 {
			return "now"
		}
		if s == 1 {
			return "1 sec"
		}
		return fmt.Sprintf("%d sec", s)
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1 min"
		}
		return fmt.Sprintf("%d min", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", h)
	}
	if d < 30*24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d < 365*24*time.Hour {
		months := int(d.Hours() / (24 * 30))
		if months <= 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	}
	years := int(d.Hours() / (24 * 365))
	if years <= 1 {
		return "1 year"
	}
	return fmt.Sprintf("%d years", years)
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
	var fetchCmd tea.Cmd     // optional async clerk fetch
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

	// Handle async user info messages
	if infoMsg, ok := msg.(app.ClerkUserInfoMsg); ok {
		incoming := strings.TrimSpace(infoMsg.Token)
		current := strings.TrimSpace(m.ClerkUserInfoToken)
		if incoming == current {
			m.ClerkUserInfo = infoMsg.Info
			m.ClerkUserInfoAttempted = true
		}
		// Always clear fetching to avoid spinner getting stuck
		m.IsFetchingClerkUserInfo = false
	}

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
					// Handle actions (logout, history, paste)
					if strings.ToLower(itemName) == "logout" {
						cfg, _ := config.LoadConfig()
						cfg.IsLoggedIn = false
						cfg.Token = ""
						_ = config.SaveConfig(cfg)
						m.IsLoggedIn = false
						m.CurrentScreen = app.ScreenLogin
						return m, nil
					} else if strings.ToLower(itemName) == "command history" {
						m.CurrentScreen = app.ScreenCommandHistory
						m.HistoryScreenIndex = 0
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

	// Trigger async fetch for Clerk user info when hovering Settings
	if m.MainScreenFocus == "action" && m.ActionIndex >= 0 && m.ActionIndex < len(actionRow) && strings.ToLower(actionRow[m.ActionIndex]) == "view settings" {
		cfg, _ := config.LoadConfig()
		tok := strings.TrimSpace(cfg.Token)
		if cfg.IsLoggedIn && tok != "" {
			// If token changed, reset cached state
			if strings.TrimSpace(m.ClerkUserInfoToken) != tok {
				m.ClerkUserInfo = ""
				m.ClerkUserInfoAttempted = false
			}
			if !m.IsFetchingClerkUserInfo && !m.ClerkUserInfoAttempted {
				m.IsFetchingClerkUserInfo = true
				m.ClerkUserInfoToken = tok
				fetchCmd = fetchClerkUserInfoCmd(tok)
			}
		}
	}

	// Return the updated model and any command from the paginator or other logic
	return m, tea.Batch(paginatorCmd, fetchCmd)
}

// updatePreview needs modification to accept the selected command name directly
func updatePreview(m app.Model, registry *project.ProjectRegistry, selectedCmdName string) app.Model {
	lowerCmd := strings.ToLower(selectedCmdName)

	// Reset previews
	m.FileTreePreview = ""
	m.StatsPreview = ""
	m.CurrentPreviewType = "none"

	switch lowerCmd {
	case "logout": // Renamed
		// Use sharedScreens.RenderProjectInfoSection and append user info
		base := sharedScreens.RenderProjectInfoSection(m, registry)
		cfg, _ := config.LoadConfig()
		userHeader := app.SubtitleStyle.Render("User") + "\n"
		status := "Logged out"
		if cfg.IsLoggedIn {
			masked := maskToken(cfg.Token)
			status = "Logged in (token " + masked + ")"
			// Try to decode JWT (no signature verification) for basic user info
			claims := parseJWTClaims(cfg.Token)
			if len(claims) > 0 {
				// Show a few common fields if present
				if sub, ok := claims["sub"].(string); ok && sub != "" {
					status += "\n  sub: " + sub
				}
				if email, ok := claims["email"].(string); ok && email != "" {
					status += "\n  email: " + email
				}
				if iss, ok := claims["iss"].(string); ok && iss != "" {
					status += "\n  iss: " + iss
				}
				if sid, ok := claims["sid"].(string); ok && sid != "" {
					status += "\n  sid: " + sid
				}
				if exp, ok := claims["exp"].(float64); ok && exp > 0 {
					status += "\n  exp: " + time.Unix(int64(exp), 0).Format(time.RFC3339)
				}
				// Show pro flag from public_metadata if present
				if pmRaw, ok := claims["public_metadata"]; ok {
					if pm, ok2 := pmRaw.(map[string]any); ok2 {
						if v, ok3 := pm["pro"]; ok3 {
							switch t := v.(type) {
							case bool:
								if t {
									status += "\n  pro: true"
								}
							case string:
								if strings.ToLower(t) == "true" {
									status += "\n  pro: true"
								}
							}
						}
					}
				}
			}
		}
		m.StatsPreview = base + "\n" + userHeader + "  " + status + "\n"
		m.CurrentPreviewType = "stats"
	case "command history":
		// Render a compact list of recent command history (generated files only), newest first
		m.CurrentPreviewType = "stats"
		header := app.SubtitleStyle.Render("Command History")
		var lines []string
		if registry != nil && m.ProjectPath != "" {
			if projectInfo, found := registry.GetProject(m.ProjectPath); found {
				hist := projectInfo.CommandHistory
				type row struct {
					when string
					name string
				}
				rows := []row{}
				for i := len(hist) - 1; i >= 0; i-- { // newest first if appended chronologically
					h := hist[i]
					if len(h.GeneratedFiles) == 0 {
						continue
					}
					when := relativeTimeShort(h.Timestamp)
					name := truncateWithEllipsis(h.Name, 42)
					rows = append(rows, row{when: when, name: name})
				}
				max := 10
				if len(rows) < max {
					max = len(rows)
				}
				gray := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
				for i := 0; i < max; i++ {
					lines = append(lines, gray.Render(rows[i].when)+"  "+rows[i].name)
				}
			}
		}
		if len(lines) == 0 {
			lines = []string{lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("No recent generated-file commands.")}
		}
		body := lipgloss.JoinVertical(lipgloss.Left, lines...)
		m.StatsPreview = lipgloss.JoinVertical(lipgloss.Left, header, "", body)
		return m
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

		// If this is a composite command (args/run), preview the first invoked subcommand
		if data, _, err := commands.LoadTemplateBytesForName(selectedCmdName, m.ProjectPath, registry); err == nil && commands.IsCompositeTemplate(data) {
			slugs, _ := commands.GetCompositeRunSlugs(data)
			if len(slugs) > 0 {
				first := slugs[0]
				keys, _ := commands.GetCommandVariableKeys(first, m.ProjectPath, registry)
				var placeholderMap map[string]string
				if len(keys) > 0 {
					placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
				} else {
					placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
				}
				if pv, perr := commands.GeneratePreviewFileTree(first, placeholderMap, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
					m.FileTreePreview = pv
					m.CurrentPreviewType = "file-tree"
					return m
				}
			}
		}

		// If this command is an auto-browse synthetic wrapper, preview the nearest JSON under its root
		if data, _, err := commands.LoadTemplateBytesForName(selectedCmdName, m.ProjectPath, registry); err == nil {
			var t struct {
				AutoBrowseRoot string `json:"autoBrowseRoot"`
			}
			if json.Unmarshal(data, &t) == nil && strings.TrimSpace(t.AutoBrowseRoot) != "" {
				root := strings.TrimSpace(t.AutoBrowseRoot)
				if nearest, ok := commands.FindFirstJSONUnder(root); ok {
					keys, _ := commands.GetCommandVariableKeys(nearest, m.ProjectPath, registry)
					var placeholderMap map[string]string
					if len(keys) > 0 {
						placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
					} else {
						placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
					}
					if b, rerr := commands.ReadEmbeddedTemplate(nearest); rerr == nil {
						if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, placeholderMap, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
							m.FileTreePreview = pv
							m.CurrentPreviewType = "file-tree"
							return m
						}
					}
				}
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
			label := truncateWithEllipsis(cmdName, 48)
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
				listBuilder.WriteString(app.HighlightStyle.Render(prefix+label) + "\n")
			} else {
				listBuilder.WriteString(app.ChoiceStyle.Render(prefix+label) + "\n")
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
	// Suppress history/status messages on the Recent Commands screen
	var statusLine string
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
var actionRow = []string{"paste from clipboard", "Command History", "Logout"}

// renderStaticActionBar creates the interactive action bar.
func renderStaticActionBar(items []string, selectedIndex int, hasFocus bool) string {
	var actionBarItems []string
	for i, val := range items {
		lowerVal := strings.ToLower(val)
		icon := "?"
		if lowerVal == "paste from clipboard" {
			icon = "Paste"
		} else if lowerVal == "command history" {
			icon = "History"
		} else if lowerVal == "logout" {
			icon = "Logout"
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
	// Maps to track added commands and avoid duplicates
	added := make(map[string]bool)
	resultList := []string{}

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
			// Hide commands that are not visible for this project
			spec := commands.GetCommandSpec(name)
			if spec.Name != "" && !commands.IsCommandVisible(spec, m.ProjectPath) {
				continue
			}
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

// maskToken returns a shortened representation of a token for display.
func maskToken(tok string) string {
	if tok == "" {
		return "(none)"
	}
	if len(tok) <= 10 {
		return tok
	}
	return tok[:6] + "…" + tok[len(tok)-4:]
}

// parseJWTClaims decodes a JWT's payload segment without verifying the signature.
// Supports Clerk desktop browser JWT as well if it's a JWT-like structure.
func parseJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	payload := parts[1]
	// JWT uses base64url without padding
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		// Try with standard encoding/padding fallback
		b2, err2 := base64.StdEncoding.DecodeString(payload)
		if err2 != nil {
			return nil
		}
		b = b2
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// getClerkUserInfoViaAPI tries to fetch user details from Clerk using CLERK_SECRET_KEY.
// It returns a formatted string with name/email if available.
func getClerkUserInfoViaAPI(token string) (string, bool) {
	secret := "sk_test_x4wDfcf3CRfAooiH3u1KBzzYHKgDzUNqUgW3Ut8Zeb"
	instance := "https://smooth-vervet-76.accounts.dev"
	if instance == "" {
		instance = "https://smooth-vervet-76.accounts.dev"
	}
	if secret == "" {
		return "", false
	}

	// Attempt to infer user ID from token (JWT sub) if present
	if claims := parseJWTClaims(token); len(claims) > 0 {
		if sub, ok := claims["sub"].(string); ok && sub != "" {
			if s, ok2 := fetchClerkUser(instance, secret, sub); ok2 {
				return s, true
			}
		}
	}

	// Fallback: try to fetch the current user via sessions API using the token if it's a session token
	if s, ok := fetchClerkMe(instance, secret, token); ok {
		return s, true
	}
	return "", false
}

func fetchClerkUser(instance, secret, userID string) (string, bool) {
	req, err := http.NewRequest("GET", instance+"/v1/users/"+userID, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	var body struct {
		ID           string `json:"id"`
		Email        string `json:"email_address"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		PrimaryEmail struct {
			EmailAddress string `json:"email_address"`
		} `json:"primary_email_address"`
		PrivateMetadata map[string]any `json:"private_metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false
	}
	email := body.Email
	if email == "" {
		email = body.PrimaryEmail.EmailAddress
	}
	name := strings.TrimSpace(body.FirstName + " " + body.LastName)
	out := "  name: " + name
	if email != "" {
		out += "\n  email: " + email
	}
	out += "\n  id: " + body.ID
	if ent, ok := extractEntitlements(body.PrivateMetadata); ok {
		out += "\n" + ent
	}
	if pm, ok := formatPrivateMetadata(body.PrivateMetadata); ok {
		out += "\n" + pm
	}
	return out, true
}

func fetchClerkMe(instance, secret, token string) (string, bool) {
	// Try to fetch the session's user
	req, err := http.NewRequest("GET", instance+"/v1/me", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	// Some Clerk deployments may require the session token as a header to resolve the subject
	if token != "" {
		req.Header.Set("X-Session-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}
	var body struct {
		ID              string         `json:"id"`
		Email           string         `json:"email_address"`
		FirstName       string         `json:"first_name"`
		LastName        string         `json:"last_name"`
		PrivateMetadata map[string]any `json:"private_metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false
	}
	name := strings.TrimSpace(body.FirstName + " " + body.LastName)
	out := "  name: " + name
	if body.Email != "" {
		out += "\n  email: " + body.Email
	}
	if body.ID != "" {
		out += "\n  id: " + body.ID
	}
	if ent, ok := extractEntitlements(body.PrivateMetadata); ok {
		out += "\n" + ent
	}
	if pm, ok := formatPrivateMetadata(body.PrivateMetadata); ok {
		out += "\n" + pm
	}
	return out, true
}

// extractEntitlements reads private_metadata.entitlements.nextgen_cli and formats it.
func extractEntitlements(privateMD map[string]any) (string, bool) {
	if privateMD == nil {
		return "", false
	}
	entitlementsRaw, ok := privateMD["entitlements"]
	if !ok {
		return "", false
	}
	entitlements, ok := entitlementsRaw.(map[string]any)
	if !ok {
		return "", false
	}
	ngRaw, ok := entitlements["nextgen_cli"]
	if !ok {
		return "", false
	}
	ng, ok := ngRaw.(map[string]any)
	if !ok {
		return "", false
	}
	// Safely pull known fields
	get := func(k string) string {
		if v, ok := ng[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return fmt.Sprintf("%g", t)
			case nil:
				return "null"
			default:
				b, _ := json.Marshal(t)
				return string(b)
			}
		}
		return ""
	}
	lines := []string{"  entitlements.nextgen_cli:",
		"    plan: " + get("plan"),
		"    status: " + get("status"),
		"    product: " + get("product"),
		"    validUntilMs: " + get("validUntilMs"),
		"    stripePriceId: " + get("stripePriceId"),
		"    stripeCustomerId: " + get("stripeCustomerId"),
		"    stripeSubscriptionId: " + get("stripeSubscriptionId"),
	}
	return strings.Join(lines, "\n"), true
}

// formatPrivateMetadata returns the full private_metadata as indented JSON.
// It prefixes the block with a label and indents the JSON for readability.

func formatPrivateMetadata(privateMD map[string]any) (string, bool) {
	if len(privateMD) == 0 {
		return "", false
	}
	b, err := json.MarshalIndent(privateMD, "", "  ")
	if err != nil {
		return "", false
	}
	// Indent JSON block under a header for consistent styling
	return "  private_metadata:\n" + indentLines("    ", string(b)), true
}

// indentLines prefixes every line in s with prefix.
func indentLines(prefix, s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// fetchClerkUserInfoCmd runs Clerk user info fetch in background and returns an async message
func fetchClerkUserInfoCmd(token string) tea.Cmd {
	return func() tea.Msg {
		if extra, ok := getClerkUserInfoViaAPI(token); ok {
			return app.ClerkUserInfoMsg{Info: extra, Token: token}
		}
		return app.ClerkUserInfoMsg{Info: "", Token: token}
	}
}
