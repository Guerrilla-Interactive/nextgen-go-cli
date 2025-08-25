package projectCmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GetSortedProjectCommandNames retrieves sorted project command names from the .nextgen/local-commands directory
func GetSortedProjectCommandNames(projectPath string) ([]string, error) {
	if projectPath == "" {
		return []string{}, nil // No project path, no local commands
	}
	localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
	entries, err := os.ReadDir(localCmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // Directory doesn't exist, no commands
		}
		return nil, fmt.Errorf("failed to read local commands dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			// Extract name from filename (remove .json)
			name := strings.TrimSuffix(entry.Name(), ".json")
			// Optionally, convert kebab-case back to something more readable if needed?
			// For now, just use the filename base.
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// updateProjectCommandPreview generates the file tree preview for the selected project command.
func updateProjectCommandPreview(m app.Model) app.Model {
	m.ProjectCommandPreview = "Loading preview..."
	projectCmdNames, _ := GetSortedProjectCommandNames(m.ProjectPath)
	totalCmds := len(projectCmdNames)
	p := m.ProjectCommandsPaginator
	start, _ := p.GetSliceBounds(totalCmds)
	numItemsOnPage := p.ItemsOnPage(totalCmds)
	isBackSelected := m.ProjectCommandsListIndex == numItemsOnPage
	var realIndex int
	if totalCmds > 0 {
		realIndex = start + m.ProjectCommandsListIndex
	} else {
		realIndex = -1
	}

	if isBackSelected || realIndex < 0 || realIndex >= totalCmds {
		m.ProjectCommandPreview = "(Select a command)"
		return m
	}

	cmdName := projectCmdNames[realIndex]
	localCmdPath := filepath.Join(m.ProjectPath, ".nextgen", "local-commands", cmdName+".json")

	// Read the local JSON file
	data, err := os.ReadFile(localCmdPath)
	if err != nil {
		m.ProjectCommandPreview = fmt.Sprintf("(Error reading %s: %v)", cmdName+".json", err)
		return m
	}

	// Unmarshal to get potential default variables (though unlikely for local commands)
	// We mostly need the structure for GeneratePreviewFileTree
	var template commands.JSONCommandTemplate
	if err := json.Unmarshal(data, &template); err != nil {
		m.ProjectCommandPreview = fmt.Sprintf("(Error parsing %s: %v)", cmdName+".json", err)
		return m
	}

	// --- Determine Placeholders ---
	// Try to infer keys from the template content first
	keys := commands.InferVariableKeys(string(data))
	var placeholderMap map[string]string
	if len(keys) > 0 {
		// Build placeholders with default <Value> style if keys found
		placeholders := make(map[string]string)
		for _, key := range keys {
			placeholders[key] = "<" + key + ">" // Use key name as placeholder
		}
		placeholderMap = commands.BuildPlaceholders(placeholders)
	} else {
		// Fallback if no keys inferred (or template has no variables)
		placeholderMap = commands.BuildAutoPlaceholders(map[string]string{"Main": "<Value>"})
	}
	// --- End Determine Placeholders ---

	// Generate the preview tree using the logic from command-helpers
	// Need a function similar to GeneratePreviewFileTree but taking bytes
	pv, err := commands.GeneratePreviewFileTreeFromBytes(data, placeholderMap, m.ProjectPath)
	if err == nil && strings.TrimSpace(pv) != "" {
		m.ProjectCommandPreview = pv
	} else {
		m.ProjectCommandPreview = fmt.Sprintf("(Preview generation failed for %s)", cmdName)
		if err != nil {
			m.ProjectCommandPreview += fmt.Sprintf("\nError: %v", err)
		}
	}
	return m
}

// UpdateScreenProjectCommandsList handles navigation for locally saved project commands.
func UpdateScreenProjectCommandsList(m app.Model, msg tea.KeyMsg, registry *project.ProjectRegistry) (app.Model, tea.Cmd) {
	projectCmdNames, err := GetSortedProjectCommandNames(m.ProjectPath)
	if err != nil {
		m.HistorySaveStatus = fmt.Sprintf("Error loading project commands: %v", err)
		projectCmdNames = []string{}
	}
	totalCmds := len(projectCmdNames)

	// --- Paginator Setup ---
	m.ProjectCommandsPaginator.SetTotalPages(totalCmds)
	p := &m.ProjectCommandsPaginator

	// --- Calculate index and page options ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back
	if m.ProjectCommandsListIndex >= numOptionsOnPage {
		m.ProjectCommandsListIndex = numOptionsOnPage - 1
	}
	if m.ProjectCommandsListIndex < 0 { // Ensure index is not negative
		m.ProjectCommandsListIndex = 0
	}
	var realIndex int // Index in the full list
	if totalCmds > 0 {
		realIndex = start + m.ProjectCommandsListIndex
	} else {
		realIndex = -1 // No items
	}
	isBackSelected := m.ProjectCommandsListIndex == numItemsOnPage

	// Update paginator first
	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "left", "h":
		if totalCmds > 0 {
			oldPage := p.Page
			*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyLeft})
			if p.Page != oldPage {
				m.ProjectCommandsListIndex = 0
				m = updateProjectCommandPreview(m)
			}
		}
		return m, paginatorCmd

	case "right", "l":
		if totalCmds > 0 {
			oldPage := p.Page
			*p, paginatorCmd = p.Update(tea.KeyMsg{Type: tea.KeyRight})
			if p.Page != oldPage {
				m.ProjectCommandsListIndex = 0
				m = updateProjectCommandPreview(m)
			}
		}
		return m, paginatorCmd

	case "up", "k":
		if numOptionsOnPage > 0 {
			newIndex := (m.ProjectCommandsListIndex + numOptionsOnPage - 1) % numOptionsOnPage
			if newIndex != m.ProjectCommandsListIndex {
				m.ProjectCommandsListIndex = newIndex
				m = updateProjectCommandPreview(m)
			}
		}

	case "down", "j":
		if numOptionsOnPage > 0 {
			newIndex := (m.ProjectCommandsListIndex + 1) % numOptionsOnPage
			if newIndex != m.ProjectCommandsListIndex {
				m.ProjectCommandsListIndex = newIndex
				m = updateProjectCommandPreview(m)
			}
		}

	case "enter":
		if isBackSelected {
			m.CurrentScreen = app.ScreenSettings
			m.ProjectCommandsListIndex = 0
			return m, nil
		} else if realIndex >= 0 && realIndex < totalCmds {
			cmdName := projectCmdNames[realIndex]
			m.SelectedProjectCommand = cmdName
			m.CurrentScreen = app.ScreenProjectCommandActions
			m.ProjectCommandActionIndex = 0
			return m, nil
		}

	case "esc", "b": // Go back to Settings
		m.CurrentScreen = app.ScreenSettings
		m.ProjectCommandsListIndex = 0
		return m, nil
	}

	return m, paginatorCmd
}

// ViewScreenProjectCommandsList renders the list of locally saved project commands.
func ViewScreenProjectCommandsList(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Project Commands (.nextgen/local-commands)") + "\n"

	projectCmdNames, err := GetSortedProjectCommandNames(m.ProjectPath)
	if err != nil {
		finalView := lipgloss.JoinVertical(lipgloss.Left, header, app.ChoiceStyle.Render(fmt.Sprintf("Error reading commands: %v", err)))
		return finalView
	}
	totalCmds := len(projectCmdNames)

	// --- Get paginated items ---
	p := m.ProjectCommandsPaginator
	start, end := p.GetSliceBounds(totalCmds)
	paginatedCmds := []string{}
	if start < end {
		paginatedCmds = projectCmdNames[start:end]
	}
	numItemsOnPage := len(paginatedCmds)
	isBackSelected := m.ProjectCommandsListIndex == numItemsOnPage

	// --- Calculate Paginator View Early ---
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}

	// --- Render List Items ---
	var listBuilder strings.Builder
	listBuilder.WriteString(app.SubtitleStyle.Render("Select Command:") + "\n\n")
	if totalCmds == 0 {
		listBuilder.WriteString(app.ChoiceStyle.Render("  (No commands saved in .nextgen/local-commands)") + "\n")
	} else {
		for i, name := range paginatedCmds {
			// Check favorite status
			prefix := "  "
			if registry != nil {
				if isFav, ok := registry.FavoriteProjectCommands[name]; ok && isFav {
					prefix = "⭐ " // Add star for favorites
				}
			}

			if i == m.ProjectCommandsListIndex {
				listBuilder.WriteString(app.HighlightStyle.Render("> "+prefix+name) + "\n")
			} else {
				listBuilder.WriteString(app.ChoiceStyle.Render("  "+prefix+name) + "\n") // Indent non-selected favorites too
			}
		}
	}
	// listBuilder now only contains command items

	// --- Render Back Button Separately ---
	backButtonView := ""
	if isBackSelected {
		backButtonView = app.HighlightStyle.Render("> Back")
	} else {
		backButtonView = app.ChoiceStyle.Render("  Back")
	}

	// --- Combine Left Pane Content ---
	leftContentItems := []string{listBuilder.String()}
	if paginatorView != "" {
		leftContentItems = append(leftContentItems, lipgloss.NewStyle().MarginTop(1).Render(paginatorView))
	}
	leftContentItems = append(leftContentItems, backButtonView)
	leftContentCombined := lipgloss.JoinVertical(lipgloss.Left, leftContentItems...)

	// --- Left Panel Styling & Rendering ---
	leftPanelWidth := 40
	leftPanel := lipgloss.NewStyle().
		Width(leftPanelWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Render(leftContentCombined)

	// --- Right Pane: File Tree Preview ---
	previewContent := m.ProjectCommandPreview
	if previewContent == "" {
		previewContent = app.HelpStyle.Render("Select a command to see its file tree preview.")
	}

	// --- Truncate Preview Content ---
	const maxPreviewLines = 12
	lines := strings.Split(previewContent, "\n")
	if len(lines) > maxPreviewLines {
		previewContent = strings.Join(lines[:maxPreviewLines], "\n")
		previewContent += "\n... (truncated)"
	}

	// Prepend header
	folderName := filepath.Base(m.ProjectPath)
	headerPreview := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContent = headerPreview + "\n\n" + previewContent

	// Define right panel style WITHOUT explicit width
	rightPanelStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Height(lipgloss.Height(leftPanel)). // Match height roughly
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
	rightPanel := rightPanelStyle.Render(previewContent)

	// --- Combine, Footer ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)
	footer := app.HelpStyle.Render("Use ↑/↓/←/→ to navigate, Enter to select, Esc/b to go back.")

	finalView := lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", footer)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}
