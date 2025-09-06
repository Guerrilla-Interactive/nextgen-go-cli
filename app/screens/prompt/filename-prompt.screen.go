package prompt

import (
    "encoding/json"
    "fmt"
    "path/filepath"
    "strings"
    "time"

	"sort"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// Toggle: enable/disable live file-tree preview while typing.
// When true, preview reflects current input (debounced) wherever possible.
const livePreviewDuringTyping = true

// Toggle: derive counterpart variables (Singular<->Plural) for preview only.
// Disable if this causes unwanted "adds an s" behavior in some templates.
const derivePreviewPairs = false

// PromptPreviewMsg is emitted after a short debounce to refresh the live preview
// using the current input for the active variable.
type PromptPreviewMsg struct{ Seq int }

func debouncePreviewCmd(seq int, d time.Duration) tea.Cmd {
    return tea.Tick(d, func(time.Time) tea.Msg {
        return PromptPreviewMsg{Seq: seq}
    })
}

// --- Preview helpers: derive missing vars (Plural/Singular) for preview only ---
func naivePluralize(s string) string {
    ls := strings.ToLower(s)
    if strings.HasSuffix(ls, "y") && len(s) > 1 {
        return s[:len(s)-1] + "ies"
    }
    if strings.HasSuffix(ls, "s") || strings.HasSuffix(ls, "sh") || strings.HasSuffix(ls, "ch") || strings.HasSuffix(ls, "x") || strings.HasSuffix(ls, "z") {
        return s + "es"
    }
    return s + "s"
}

func naiveSingularize(s string) string {
    ls := strings.ToLower(s)
    if strings.HasSuffix(ls, "ies") && len(s) > 3 {
        return s[:len(s)-3] + "y"
    }
    if strings.HasSuffix(ls, "es") && len(s) > 2 {
        // crude: remove es
        return s[:len(s)-2]
    }
    if strings.HasSuffix(ls, "s") && len(s) > 1 {
        return s[:len(s)-1]
    }
    return s
}

func baseOf(key string) (string, string) {
    if strings.HasSuffix(key, "Plural") {
        return strings.TrimSuffix(key, "Plural"), "Plural"
    }
    if strings.HasSuffix(key, "Singular") {
        return strings.TrimSuffix(key, "Singular"), "Singular"
    }
    return key, ""
}

func addDerivedPreviewVars(varKeys []string, vars map[string]string, activeKey string) map[string]string {
    // derive missing Plural/Singular variants based on available counterparts
    out := make(map[string]string, len(vars)+4)
    for k, v := range vars {
        out[k] = v
    }
    // quick lookup of available values
    has := func(k string) (string, bool) { v, ok := out[k]; return v, ok }
    // limit derivation to the active key family (reduces chaos when many vars)
    activeBase, _ := baseOf(activeKey)
    for _, k := range varKeys {
        if _, ok := out[k]; ok {
            continue
        }
        base, suf := baseOf(k)
        if activeBase != "" && base != activeBase {
            // do not derive unrelated families
            continue
        }
        if suf == "Plural" {
            base := strings.TrimSuffix(k, "Plural")
            if v, ok := has(base+"Singular"); ok && strings.TrimSpace(v) != "" {
                out[k] = naivePluralize(v)
                continue
            }
            if v, ok := has(base); ok && strings.TrimSpace(v) != "" {
                out[k] = naivePluralize(v)
                continue
            }
        }
        if suf == "Singular" {
            base := strings.TrimSuffix(k, "Singular")
            if v, ok := has(base+"Plural"); ok && strings.TrimSpace(v) != "" {
                out[k] = naiveSingularize(v)
                continue
            }
            if v, ok := has(base); ok && strings.TrimSpace(v) != "" {
                // assume already singular
                out[k] = v
                continue
            }
        }
    }
    return out
}

// UpdateScreenChoicePrompt handles a simple two-option choice with preview and back.
func UpdateScreenChoicePrompt(m app.Model, keyMsg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	switch keyMsg.String() {
	case "esc":
		m.CurrentScreen = app.ScreenMain
		m.FileTreePreview = ""
		m.CurrentPreviewType = "none"
		return m, nil
	case "up", "k":
		if m.PromptOptionFocused {
			// Stay on Back when pressing up
			return m, nil
		}
		if m.ChoiceIndex == 0 {
			// Move focus to Back when at top of list
			m.PromptOptionFocused = true
		} else if m.ChoiceIndex > 0 {
			m.ChoiceIndex--
		}
	case "down", "j":
		if m.PromptOptionFocused {
			// From Back, return focus to the list (first item stays selected)
			m.PromptOptionFocused = false
			return m, nil
		}
		if m.ChoiceIndex < len(m.ChoiceOptionNames)-1 {
			m.ChoiceIndex++
		}
	case "enter":
		if m.PromptOptionFocused {
			m.CurrentScreen = app.ScreenMain
			m.FileTreePreview = ""
			m.CurrentPreviewType = "none"
			return m, nil
		}
		// Picked subcommand; go to filename prompt
		if m.ChoiceIndex >= 0 && m.ChoiceIndex < len(m.ChoiceTargetSlugs) {
			target := m.ChoiceTargetSlugs[m.ChoiceIndex]
			// Auto-browse: if target looks like an embedded folder (ends with '/'), drill down
			if strings.HasSuffix(target, "/") && strings.HasPrefix(target, "native-commands/") {
				prefix := strings.TrimSuffix(target, "/")
				children, err := commands.ListNativeChildren(prefix)
				if err == nil && len(children) > 0 {
					// If only one child and it's a dir, auto-drill further
					if len(children) == 1 && children[0].IsDir {
						m.ChoiceOptionNames = []string{children[0].Name + "/"}
						m.ChoiceTargetSlugs = []string{children[0].Path + "/"}
						m.ChoiceIndex = 0
						return m, nil
					}
					m.ChoiceOptionNames = []string{}
					m.ChoiceTargetSlugs = []string{}
					for _, c := range children {
						if c.IsDir {
							m.ChoiceOptionNames = append(m.ChoiceOptionNames, c.Name+"/")
							m.ChoiceTargetSlugs = append(m.ChoiceTargetSlugs, c.Path+"/")
						} else {
							// JSON file: proceed to prompt
            m.PendingCommand = c.Path
            keys, _ := commands.GetCommandVariableKeys(c.Path, m.ProjectPath, registry)
							// Order keys by template priority (lowest first)
							if pr, _ := commands.GetCommandVariablePriorities(c.Path, m.ProjectPath, registry); len(pr) > 0 {
								keys = orderKeysByPriority(keys, pr)
							}
            m.MultipleVariables = len(keys) > 1
            m.VariableKeys = keys
            m.CurrentVariableIndex = 0
            m.Variables = make(map[string]string)
            m.TempFilename = ""
            // Initialize preview before switching screens for stability
            m = updateFilenamePromptPreview(m, registry)
            m.CurrentScreen = app.ScreenFilenamePrompt
            return m, cursor.Blink
						}
					}
					m.ChoiceIndex = 0
					return m, nil
				}
			}
            m.PendingCommand = target
            keys, _ := commands.GetCommandVariableKeys(target, m.ProjectPath, registry)
			if pr, _ := commands.GetCommandVariablePriorities(target, m.ProjectPath, registry); len(pr) > 0 {
				keys = orderKeysByPriority(keys, pr)
			}
            m.MultipleVariables = len(keys) > 1
            m.VariableKeys = keys
            m.CurrentVariableIndex = 0
            m.Variables = make(map[string]string)
            m.TempFilename = ""
            // Initialize preview before switching screens for stability
            m = updateFilenamePromptPreview(m, registry)
            m.CurrentScreen = app.ScreenFilenamePrompt
            return m, cursor.Blink
		}
	}
	// Update preview for current choice (only when list has focus)
	if !m.PromptOptionFocused && m.ChoiceIndex >= 0 && m.ChoiceIndex < len(m.ChoiceTargetSlugs) {
		sel := m.ChoiceTargetSlugs[m.ChoiceIndex]
		// Folder path: preview nearest JSON inside
		if strings.HasPrefix(sel, "native-commands/") && strings.HasSuffix(sel, "/") {
			if nearest, ok := commands.FindFirstJSONUnder(strings.TrimSuffix(sel, "/")); ok {
				keys, _ := commands.GetCommandVariableKeys(nearest, m.ProjectPath, registry)
				var ph map[string]string
				if len(keys) > 0 {
					ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
				} else {
					ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
				}
				if b, rerr := commands.ReadEmbeddedTemplate(nearest); rerr == nil {
					if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
						m.FileTreePreview = pv
						m.CurrentPreviewType = "file-tree"
						return m, nil
					}
				}
			}
		}
		// File path in embedded tree
		if strings.HasPrefix(sel, "native-commands/") && strings.HasSuffix(sel, ".json") {
			keys, _ := commands.GetCommandVariableKeys(sel, m.ProjectPath, registry)
			if pr, _ := commands.GetCommandVariablePriorities(sel, m.ProjectPath, registry); len(pr) > 0 {
				keys = orderKeysByPriority(keys, pr)
			}
			var ph map[string]string
			if len(keys) > 0 {
				ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
			} else {
				ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
			}
			if b, rerr := commands.ReadEmbeddedTemplate(sel); rerr == nil {
				if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
					m.FileTreePreview = pv
					m.CurrentPreviewType = "file-tree"
					return m, nil
				}
			}
		}
		// Fallback as command name/slug
		keys, _ := commands.GetCommandVariableKeys(sel, m.ProjectPath, registry)
		if pr, _ := commands.GetCommandVariablePriorities(sel, m.ProjectPath, registry); len(pr) > 0 {
			keys = orderKeysByPriority(keys, pr)
		}
		var ph map[string]string
		if len(keys) > 0 {
			ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
		} else {
			ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
		}
		pv, err := commands.GeneratePreviewFileTree(sel, ph, m.ProjectPath)
		if err == nil && strings.TrimSpace(pv) != "" {
			m.FileTreePreview = pv
			m.CurrentPreviewType = "file-tree"
		}
	}
	return m, nil
}

func ViewChoicePrompt(m app.Model, registry *project.ProjectRegistry) string {
	// Left panel with [Back] above list; both bottom-aligned
	var backBtn string
	if m.PromptOptionFocused {
		backBtn = app.HighlightStyle.Render("[Back]")
	} else {
		backBtn = app.HelpStyle.Render("[Back]")
	}
	var listBuilder strings.Builder
	for i, name := range m.ChoiceOptionNames {
		if !m.PromptOptionFocused && i == m.ChoiceIndex {
			listBuilder.WriteString(app.HighlightStyle.Render(name) + "\n")
		} else {
			listBuilder.WriteString(app.ChoiceStyle.Render(name) + "\n")
		}
	}
	// Favor a wider left column; allow right to overflow
	gap := 1
	leftPanelWidth := sharedScreens.ComputeLeftPanelWidthFavorLeft(m.TerminalWidth)
	// Footer navigation tips (use arrows)
	footer := sharedScreens.Footer("↑↓ ←→ navigate", "enter to confirm", "ctrl+c quit")
	footerHeight := lipgloss.Height(footer)
	availableHeight := m.TerminalHeight - footerHeight - 1
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Right panel: preview with header and truncation; bottom-left alignment
	previewHeader := sharedScreens.ProjectHeader(m.ProjectPath)
	rawPreview := m.FileTreePreview
	if strings.TrimSpace(rawPreview) == "" {
		// If we have multiple-choice slugs, preview the first
		if len(m.ChoiceTargetSlugs) > 0 {
			first := m.ChoiceTargetSlugs[0]
			// If first is an embedded folder, preview nearest JSON
			if strings.HasPrefix(first, "native-commands/") && strings.HasSuffix(first, "/") {
				if nearest, ok := commands.FindFirstJSONUnder(strings.TrimSuffix(first, "/")); ok {
					keys, _ := commands.GetCommandVariableKeys(nearest, m.ProjectPath, registry)
					var ph map[string]string
					if len(keys) > 0 {
						ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
					} else {
						ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
					}
					if b, rerr := commands.ReadEmbeddedTemplate(nearest); rerr == nil {
						if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
							rawPreview = pv
						} else {
							rawPreview = "No preview available for this command."
						}
					} else {
						rawPreview = "No preview available for this command."
					}
				} else {
					rawPreview = "No preview available for this command."
				}
			} else if strings.HasPrefix(first, "native-commands/") && strings.HasSuffix(first, ".json") {
				// First is an embedded JSON path
				keys, _ := commands.GetCommandVariableKeys(first, m.ProjectPath, registry)
				var ph map[string]string
				if len(keys) > 0 {
					ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
				} else {
					ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
				}
				if b, rerr := commands.ReadEmbeddedTemplate(first); rerr == nil {
					if pv, perr := commands.GeneratePreviewFileTreeFromBytes(b, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
						rawPreview = pv
					} else {
						rawPreview = "No preview available for this command."
					}
				} else {
					rawPreview = "No preview available for this command."
				}
			} else {
				// Fallback to name/slug based preview
				keys, _ := commands.GetCommandVariableKeys(first, m.ProjectPath, registry)
				var ph map[string]string
				if len(keys) > 0 {
					ph = commands.BuildPlaceholders(map[string]string{keys[0]: "<" + keys[0] + ">"})
				} else {
					ph = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
				}
				if pv, perr := commands.GeneratePreviewFileTree(first, ph, m.ProjectPath); perr == nil && strings.TrimSpace(pv) != "" {
					rawPreview = pv
				} else {
					rawPreview = "No preview available for this command."
				}
			}
		} else {
			rawPreview = "No preview available for this command."
		}
	}
	headerHeight := 2
	padding := 2
	maxPreviewContent := availableHeight - headerHeight - padding
	if maxPreviewContent < 1 {
		maxPreviewContent = 1
	}
	rawPreview = sharedScreens.TruncateLines(rawPreview, maxPreviewContent)
	// Build left column: bottom-align a vertical stack [Back, list]
	stack := lipgloss.JoinVertical(lipgloss.Left, backBtn, listBuilder.String())
	leftContentWidth := leftPanelWidth - 2 // account for left/right padding = 1 each
	if leftContentWidth < 0 {
		leftContentWidth = 0
	}
	leftPanel := lipgloss.NewStyle().Padding(0, 1).Width(leftContentWidth).Render(stack)
	leftPlaced := lipgloss.Place(leftPanelWidth, availableHeight, lipgloss.Left, lipgloss.Bottom, leftPanel)

	// Let right panel size to content (may exceed screen width)
	rightInner := lipgloss.NewStyle().Padding(1, 1).Render(previewHeader + "\n\n" + rawPreview)
	rightPanel := lipgloss.Place(lipgloss.Width(rightInner), availableHeight, lipgloss.Left, lipgloss.Bottom, rightInner)

	gapSpacing := strings.Repeat(" ", gap)
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPlaced, gapSpacing, rightPanel)
	final := lipgloss.JoinVertical(lipgloss.Left, combined, "\n")
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, final)
	}
	return final
}

// UpdateScreenFilenamePrompt handles input for both single and multiple variables.
// It now accepts the registry to pass down to RunCommand.
func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	// Check for arrow keys (actual arrow keys) to change focus.
	switch keyMsg.String() {
	case "up":
		// Pressing the up arrow focuses the "[Back]" button.
		m.PromptOptionFocused = true
		return m, nil
	case "down":
		// Pressing the down arrow returns focus to the input field.
		m.PromptOptionFocused = false
		return m, nil
	}

	// If the "[Back]" button is focused, process only the Enter key.
	if m.PromptOptionFocused {
		if keyMsg.String() == "enter" {
			m.CurrentScreen = app.ScreenMain
			m.TempFilename = ""
			m.FileTreePreview = ""
			m.StatsPreview = ""
			m.CurrentPreviewType = "none"
			m.PromptOptionFocused = false
		}
		return m, nil
	}

	// Check if we are in multi-variable mode.
	if m.MultipleVariables {
		switch keyMsg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			// Go back to recent commands.
			m.CurrentScreen = app.ScreenMain
			m.TempFilename = ""
			m.FileTreePreview = ""
			m.StatsPreview = ""
			m.CurrentPreviewType = "none"
			// Also ensure we are not in prompt option focus.
			m.PromptOptionFocused = false
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.TempFilename)
			if value == "" {
				return m, nil
			}
			currentKey := m.VariableKeys[m.CurrentVariableIndex]
			m.Variables[currentKey] = value
			m.TempFilename = ""
			m.CurrentVariableIndex++

			if m.CurrentVariableIndex >= len(m.VariableKeys) {
				// Record only the base command (not inputs) to recent
				sharedScreens.RecordCommand(&m, m.PendingCommand)
				mainValue := m.Variables[m.VariableKeys[0]]
				extraVars := make(map[string]string)
				for i := 0; i < len(m.VariableKeys); i++ {
					key := m.VariableKeys[i]
					extraVars[key] = m.Variables[key]
				}
				placeholders := commands.BuildMultiPlaceholders(mainValue, extraVars)
				// --- DEBUG ---
				fmt.Printf("DEBUG [Multi-Var Enter]: Placeholders: %+v\n", placeholders)
				// -------------
				m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", m.PendingCommand)
				m.CurrentScreen = app.ScreenInstallDetails
				runCmd := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholders, registry)
				return m, runCmd
			} else {
				// Still more variables to collect, update preview for next prompt
				m = updateFilenamePromptPreview(m, registry)
				return m, cursor.Blink // Return blink for the next input field
			}
        case "backspace":
            if len(m.TempFilename) > 0 {
                m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
            }
            if livePreviewDuringTyping {
                // Schedule debounced live preview update
                m.PromptPreviewSeq++
                m.PromptPreviewPending = true
                return m, tea.Batch(cursor.Blink, debouncePreviewCmd(m.PromptPreviewSeq, 150*time.Millisecond))
            }
            return m, cursor.Blink
        default:
            // Append single character inputs.
            if len(keyMsg.String()) == 1 {
                m.TempFilename += keyMsg.String()
            }
            if livePreviewDuringTyping {
                // Schedule debounced live preview update
                m.PromptPreviewSeq++
                m.PromptPreviewPending = true
                return m, tea.Batch(cursor.Blink, debouncePreviewCmd(m.PromptPreviewSeq, 150*time.Millisecond))
            }
            return m, cursor.Blink
		}
	}
	// Single variable mode.
	switch keyMsg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		// Go back to recent commands.
		m.CurrentScreen = app.ScreenMain
		m.TempFilename = ""
		m.FileTreePreview = ""
		m.StatsPreview = ""
		m.CurrentPreviewType = "none"
		return m, nil
	case "enter":
		filename := strings.TrimSpace(m.TempFilename)
		if filename == "" {
			return m, nil
		}

		// Determine the placeholder map.
		keys, err := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
		var placeholderMap map[string]string
		if err == nil && len(keys) > 0 {
			placeholderMap = commands.BuildPlaceholders(map[string]string{keys[0]: filename})
		} else {
			if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Filename": filename})
			} else {
				placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": filename})
			}
		}
		// --- DEBUG ---
		fmt.Printf("DEBUG [Single-Var Enter]: Placeholders: %+v\n", placeholderMap)
		// -------------

		// --- Handle Clipboard Saving Separately (Before Running Command) ---
		if strings.ToLower(m.PendingCommand) == "paste from clipboard" && filename != "" {
			if registry != nil {
				commandNameToSave := filename // Default
				clipboardContent, readErr := clipboard.ReadAll()
				if readErr == nil {
					type cmdMeta struct {
						Title string `json:"title"`
						Type  string `json:"_type"`
					}
					var meta cmdMeta
					if json.Unmarshal([]byte(clipboardContent), &meta) == nil && meta.Type == "command" && meta.Title != "" {
						commandNameToSave = meta.Title
					}
				}

				clipboardContentToSave := clipboardContent // Use content read above if possible
				if clipboardContentToSave == "" && readErr != nil {
					clipboardContentToSave, _ = clipboard.ReadAll() // Try reading again
				}
				if clipboardContentToSave != "" {
					if err := commands.UpsertClipboardCommand(registry, commandNameToSave, clipboardContentToSave); err != nil {
						m.HistorySaveStatus = fmt.Sprintf("Warning: Failed to save clipboard command: %v", err)
					} else {
						m.HistorySaveStatus = fmt.Sprintf("Saved clipboard as command: %s", commandNameToSave)
					}
				} else {
					// Failed to read clipboard for saving
					m.HistorySaveStatus = "Warning: Could not read clipboard to save command."
				}
			}
		}
		// --- End Clipboard Saving ---

		// Record only the base command (not inputs) to recent
		sharedScreens.RecordCommand(&m, m.PendingCommand)
		// Set status and screen before returning command
		if m.HistorySaveStatus == "" { // Don't overwrite clipboard save status unless empty
			m.HistorySaveStatus = fmt.Sprintf("Running command: %s...", m.PendingCommand)
		}
		m.CurrentScreen = app.ScreenInstallDetails

		// Get the command to run
		runCmd := commands.RunCommand(m.PendingCommand, m.ProjectPath, placeholderMap, registry)
		return m, runCmd
	}
	// If the key is a single character (and not one of our reserved navigation keys),
	// then append it to the input. This lets you use letters (or digits, etc.) for the input.
    if len(keyMsg.String()) == 1 {
        m.TempFilename += keyMsg.String()
    } else if keyMsg.String() == "backspace" && len(m.TempFilename) > 0 {
        m.TempFilename = m.TempFilename[:len(m.TempFilename)-1]
    }

    if livePreviewDuringTyping {
        // Schedule debounced live preview update
        m.PromptPreviewSeq++
        m.PromptPreviewPending = true
        return m, tea.Batch(cursor.Blink, debouncePreviewCmd(m.PromptPreviewSeq, 150*time.Millisecond))
    }
    return m, cursor.Blink
}

// ViewFilenamePrompt displays the proper prompt based on the current mode.
func ViewFilenamePrompt(m app.Model, registry *project.ProjectRegistry) string {
	// Define a cursor style and determine whether to show the input cursor
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	inputCursor := ""
	// blink the cursor if the input field is focused

	if !m.PromptOptionFocused {
		inputCursor = cursorStyle.Render("▎")
		cursor.Blink()

	}

	// Build title + description above input box
	var label string
	var title string
	var desc string
	var inputLine string
    if m.MultipleVariables {
        if m.CurrentVariableIndex >= len(m.VariableKeys) {
            label = "Processing command..."
            inputLine = "please wait."
        } else {
            currentKey := m.VariableKeys[m.CurrentVariableIndex]
            // Prefer Title from template; fall back to "Enter <variable>" when absent
            if titleMap, _ := commands.GetCommandVariableTitles(m.PendingCommand, m.ProjectPath, registry); len(titleMap) > 0 {
                if t, ok := titleMap[currentKey]; ok {
                    title = strings.TrimSpace(t)
                }
            }
            if strings.TrimSpace(title) == "" {
                label = fmt.Sprintf("Enter %s:", currentKey)
            }
            if descMap, _ := commands.GetCommandVariableDescriptions(m.PendingCommand, m.ProjectPath, registry); len(descMap) > 0 {
                if d, ok := descMap[currentKey]; ok {
                    desc = strings.TrimSpace(d)
                }
            }
            inputLine = "> " + m.TempFilename + inputCursor
        }
    } else {
        if len(m.VariableKeys) > 0 {
            key := m.VariableKeys[0]
            // Prefer Title from template; fall back to "Enter <variable>" when absent
            if titleMap, _ := commands.GetCommandVariableTitles(m.PendingCommand, m.ProjectPath, registry); len(titleMap) > 0 {
                if t, ok := titleMap[key]; ok {
                    title = strings.TrimSpace(t)
                }
            }
            if strings.TrimSpace(title) == "" {
                label = fmt.Sprintf("Enter %s:", key)
            }
            if descMap, _ := commands.GetCommandVariableDescriptions(m.PendingCommand, m.ProjectPath, registry); len(descMap) > 0 {
                if d, ok := descMap[key]; ok {
                    desc = strings.TrimSpace(d)
                }
            }
        } else {
            label = "Enter the new file/component name:"
        }
        inputLine = "> " + m.TempFilename + inputCursor
    }

	// Build the input panel with a border that changes based on focus.
	var inputBorderStyle lipgloss.Style
	if m.PromptOptionFocused {
		// [Back] is focused so render the input field "blurred" with a gray border.
		inputBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)
	} else {
		// When the input field is focused, use a white border with extra padding.
		inputBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("15")).
			Padding(1, 2)
	}
	// Favor a wider left column; allow right to overflow
	gap := 1
	leftPanelWidth := sharedScreens.ComputeLeftPanelWidthFavorLeft(m.TerminalWidth)

	// Compose title (if any), label + optional description + input box
	var titleView string
	if strings.TrimSpace(title) != "" {
		titleView = app.TitleStyle.Render(title)
	}
	labelView := app.SubtitleStyle.Render(label)
	var descView string
	if strings.TrimSpace(desc) != "" {
		// Word-wrap the description to the available width, then style it
		available := leftPanelWidth - 2
		if available < 10 {
			available = leftPanelWidth
		}
		wrappedText := sharedScreens.WrapText(desc, available)
		descView = lipgloss.NewStyle().
			Margin(0, 0, 1, 0). // add bottom margin below description
			Render(app.HelpStyle.Render(wrappedText))
	}
	// Give the input panel a wider content area (account for border + padding)
	contentWidth := leftPanelWidth - 4
	if contentWidth < 10 {
		contentWidth = leftPanelWidth - 2
	}
	inputPanel := inputBorderStyle.Width(contentWidth).Render(inputLine)
	// Render examples under the input when available from the template
	var examplesView string
	if exMap, _ := commands.GetCommandVariableExamples(m.PendingCommand, m.ProjectPath, registry); len(exMap) > 0 {
		var exKey string
		if m.MultipleVariables {
			if m.CurrentVariableIndex < len(m.VariableKeys) {
				exKey = m.VariableKeys[m.CurrentVariableIndex]
			}
		} else if len(m.VariableKeys) > 0 {
			exKey = m.VariableKeys[0]
		}
		if exKey != "" {
			if list, ok := exMap[exKey]; ok && len(list) > 0 {
				// wrap examples to column width
				exWidth := leftPanelWidth - 2
				if exWidth < 10 {
					exWidth = leftPanelWidth
				}
				exText := "Examples: " + strings.Join(list, ", ")
				exWrapped := sharedScreens.WrapText(exText, exWidth)
				examplesView = lipgloss.NewStyle().
					Margin(0, 0, 1, 0). // add bottom margin below examples
					Render(app.HelpStyle.Render(exWrapped))
			}
		}
	}
	promptStack := labelView
	if descView != "" {
		if titleView != "" {
			if examplesView != "" {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, titleView, labelView, descView, inputPanel, examplesView)
			} else {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, titleView, labelView, descView, inputPanel)
			}
		} else {
			if examplesView != "" {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, labelView, descView, inputPanel, examplesView)
			} else {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, labelView, descView, inputPanel)
			}
		}
	} else {
		if titleView != "" {
			if examplesView != "" {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, titleView, labelView, inputPanel, examplesView)
			} else {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, titleView, labelView, inputPanel)
			}
		} else {
			if examplesView != "" {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, labelView, inputPanel, examplesView)
			} else {
				promptStack = lipgloss.JoinVertical(lipgloss.Left, labelView, inputPanel)
			}
		}
	}

	// Build the "[Back]" button row. It is highlighted when focused.
	var backButton string
	if m.PromptOptionFocused {
		backButton = app.HighlightStyle.Render("[Back]")
	} else {
		backButton = app.HelpStyle.Render("[Back]")
	}

	// Place the "[Back]" button above the input panel, with margin below it.
	backWithMargin := lipgloss.NewStyle().Margin(0, 0, 1, 0).Render(backButton)
	leftPanel := lipgloss.NewStyle().
		Width(leftPanelWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, backWithMargin, promptStack))

	// If LivePreview is empty, compute a default preview using default placeholder values.
    preview := m.FileTreePreview
    if strings.TrimSpace(preview) == "" {
        // Build placeholders consistently; include current input + derived pairs when allowed
        keys, _ := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
        if len(m.VariableKeys) > 0 {
            keys = m.VariableKeys
        }
        var placeholderMap map[string]string
        if m.MultipleVariables {
            raw := make(map[string]string)
            for i, key := range keys {
                if val, ok := m.Variables[key]; ok && strings.TrimSpace(val) != "" {
                    raw[key] = val
                } else if livePreviewDuringTyping && i == m.CurrentVariableIndex && strings.TrimSpace(m.TempFilename) != "" {
                    raw[key] = m.TempFilename
                }
            }
            if derivePreviewPairs {
                activeKey := ""
                if m.CurrentVariableIndex >= 0 && m.CurrentVariableIndex < len(keys) {
                    activeKey = keys[m.CurrentVariableIndex]
                }
                raw = addDerivedPreviewVars(keys, raw, activeKey)
            }
            for _, key := range keys {
                if _, ok := raw[key]; !ok {
                    raw[key] = "<" + key + ">"
                }
            }
            if len(keys) > 0 {
                if mv, ok := raw[keys[0]]; ok {
                    raw["Main"] = mv
                }
            }
            placeholderMap = commands.BuildPlaceholders(raw)
        } else {
            variableName := "Value"
            if len(keys) > 0 {
                variableName = keys[0]
            }
            val := "<" + variableName + ">"
            if livePreviewDuringTyping {
                if v := strings.TrimSpace(m.TempFilename); v != "" {
                    val = v
                }
            }
            raw := map[string]string{variableName: val, "Main": val}
            if derivePreviewPairs {
                raw = addDerivedPreviewVars(keys, raw, variableName)
            }
            placeholderMap = commands.BuildPlaceholders(raw)
        }
        if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
            if pv, err := commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath); err == nil && strings.TrimSpace(pv) != "" {
                preview = pv
            }
        } else {
            if pv, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath); err == nil && strings.TrimSpace(pv) != "" {
                preview = pv
            }
        }
    }

	// Prepend header with package icon and current folder name.
	folderName := filepath.Base(m.ProjectPath)
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	preview = header + "\n\n" + preview

	// Let right panel size to content (may exceed screen width)
	rightInner := lipgloss.NewStyle().Padding(1, 2).Render(preview)
	rightPanel := rightInner

	// Join the panels horizontally
	// Lipgloss should handle distributing the width
	gapSpacing := strings.Repeat(" ", gap)
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Bottom, leftPanel, gapSpacing, rightPanel)

	finalView := lipgloss.JoinVertical(lipgloss.Left,
		combinedPanes, // Use the combined panel layout
		// sharedScreens.Footer("↑↓ ←→ navigate", "enter to confirm", "ctrl+c quit"),
	)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}

// orderKeysByPriority sorts keys by ascending priority from map; unknown keys retain
// relative order but are placed after known priorities.
func orderKeysByPriority(keys []string, priorities map[string]int) []string {
	type item struct {
		key  string
		idx  int
		prio int
		has  bool
	}
	items := make([]item, len(keys))
	for i, k := range keys {
		pr, ok := priorities[k]
		items[i] = item{key: k, idx: i, prio: pr, has: ok}
	}
	sort.SliceStable(items, func(i, j int) bool {
		ai, aj := items[i], items[j]
		if ai.has && aj.has {
			if ai.prio != aj.prio {
				return ai.prio < aj.prio
			}
			return ai.idx < aj.idx
		}
		if ai.has != aj.has {
			return ai.has
		} // known before unknown
		return ai.idx < aj.idx
	})
	out := make([]string, len(keys))
	for i, it := range items {
		out[i] = it.key
	}
	return out
}

// updateFilenamePromptPreview generates the file tree preview for the filename prompt screen.
// Moved here from shared/screen-helpers.go and made internal.
func updateFilenamePromptPreview(m app.Model, registry *project.ProjectRegistry) app.Model {
    var placeholderMap map[string]string

    // Determine the correct keys first using the new function
    keys, err := commands.GetCommandVariableKeys(m.PendingCommand, m.ProjectPath, registry)
    if err != nil {
        // Handle error getting keys, maybe set preview to error message?
        m.FileTreePreview = fmt.Sprintf("Error getting keys for preview: %v", err)
        m.CurrentPreviewType = "none"
        return m
    }
    // Prefer model order if available (may already be priority-ordered)
    if len(m.VariableKeys) > 0 {
        keys = m.VariableKeys
    }

    if m.MultipleVariables {
        rawVars := make(map[string]string)
        for i, key := range keys {
            if val, ok := m.Variables[key]; ok && val != "" {
                rawVars[key] = val
            } else if livePreviewDuringTyping && i == m.CurrentVariableIndex && strings.TrimSpace(m.TempFilename) != "" {
                rawVars[key] = m.TempFilename
            }
        }
        // derive preview-only values (e.g., Plural from Singular) limited to active family
        if derivePreviewPairs {
            activeKey := ""
            if m.CurrentVariableIndex >= 0 && m.CurrentVariableIndex < len(keys) {
                activeKey = keys[m.CurrentVariableIndex]
            }
            rawVars = addDerivedPreviewVars(keys, rawVars, activeKey)
        }
        // fill remaining as placeholders
        for _, key := range keys {
            if _, ok := rawVars[key]; !ok {
                rawVars[key] = "<" + key + ">"
            }
        }
        // set Main alias to the first key's value if present
        if len(keys) > 0 {
            if mv, ok := rawVars[keys[0]]; ok {
                rawVars["Main"] = mv
            }
        }
        placeholderMap = commands.BuildPlaceholders(rawVars)
    } else {
        variableName := "Value"
        if len(keys) > 0 {
            variableName = keys[0]
        }
        val := "<" + variableName + ">"
        if livePreviewDuringTyping {
            if v := strings.TrimSpace(m.TempFilename); v != "" {
                val = v
            }
        }
        raw := map[string]string{variableName: val, "Main": val}
        // also derive potential counterpart (e.g., if key ends with Singular/Plural)
        if derivePreviewPairs {
            raw = addDerivedPreviewVars(keys, raw, variableName)
        }
        placeholderMap = commands.BuildPlaceholders(raw)
    }

	// --- Generate preview using the correct function ---
	var pv string
	var previewErr error
	if strings.ToLower(m.PendingCommand) == "paste from clipboard" {
		pv, previewErr = commands.GeneratePreviewFileTreeFromClipboard(placeholderMap, m.ProjectPath)
	} else {
		pv, previewErr = commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath)
	}
	// --- End preview generation ---

    if previewErr == nil && strings.TrimSpace(pv) != "" {
        m.FileTreePreview = pv
        m.CurrentPreviewType = "file-tree"
    } // else: keep previous preview to avoid flicker

    return m
}

// UpdateScreenFilenamePromptPreview handles the debounced preview tick. It only updates
// when the sequence matches the latest input sequence to avoid outdated updates.
func UpdateScreenFilenamePromptPreview(m app.Model, msg PromptPreviewMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
    if msg.Seq != m.PromptPreviewSeq {
        // Outdated tick; ignore
        return m, nil
    }
    // Generate using live input for the active variable
    m = updateFilenamePromptPreview(m, registry)
    m.PromptPreviewPending = false
    return m, nil
}
