package screens

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

// Helper to get sorted project command names from the .nextgen/local-commands directory
func getSortedProjectCommandNames(projectPath string) ([]string, error) {
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
	projectCmdNames, _ := getSortedProjectCommandNames(m.ProjectPath) // Ignore error for preview
	totalCmds := len(projectCmdNames)
	start, end := m.ProjectCommandsPaginator.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start // Calculate items on page consistently
	realIndex := start + m.ProjectCommandsListIndex
	isBackSelected := m.ProjectCommandsListIndex == numItemsOnPage // Use consistent calculation

	if isBackSelected || realIndex >= totalCmds {
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
	projectCmdNames, err := getSortedProjectCommandNames(m.ProjectPath)
	if err != nil {
		m.HistorySaveStatus = fmt.Sprintf("Error loading project commands: %v", err)
		projectCmdNames = []string{}
	}
	totalCmds := len(projectCmdNames)

	// --- Paginator Setup ---
	m.ProjectCommandsPaginator.SetTotalPages(totalCmds)
	p := &m.ProjectCommandsPaginator

	// --- Update Paginator ---
	// We forward the message to the paginator first
	var paginatorCmd tea.Cmd
	*p, paginatorCmd = p.Update(msg)

	// --- Calculate index and page options (needed for view/selection) ---
	start, end := p.GetSliceBounds(totalCmds)
	numItemsOnPage := end - start
	numOptionsOnPage := numItemsOnPage + 1 // Items + Back
	// Clamp list index BEFORE using it
	if m.ProjectCommandsListIndex >= numOptionsOnPage {
		m.ProjectCommandsListIndex = numOptionsOnPage - 1
	}
	realIndex := start + m.ProjectCommandsListIndex
	isBackSelected := m.ProjectCommandsListIndex == numItemsOnPage

	// --- Handle Keypresses (after paginator update) ---
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	// Left/Right are handled by the paginator's Update above
	// We just need to return its potential command

	case "up", "k":
		newIndex := (m.ProjectCommandsListIndex + numOptionsOnPage - 1) % numOptionsOnPage
		if newIndex != m.ProjectCommandsListIndex {
			m.ProjectCommandsListIndex = newIndex
			m = updateProjectCommandPreview(m) // Update preview on selection change
		}

	case "down", "j":
		newIndex := (m.ProjectCommandsListIndex + 1) % numOptionsOnPage
		if newIndex != m.ProjectCommandsListIndex {
			m.ProjectCommandsListIndex = newIndex
			m = updateProjectCommandPreview(m) // Update preview on selection change
		}

	case "enter":
		if isBackSelected { // Back selected
			m.CurrentScreen = app.ScreenProjectStats
			m.ProjectCommandsListIndex = 0
			return m, nil
		} else if realIndex < totalCmds { // Check against total commands
			cmdName := projectCmdNames[realIndex] // Potential crash point

			m.SelectedProjectCommand = cmdName
			m.CurrentScreen = app.ScreenProjectCommandActions
			m.ProjectCommandActionIndex = 0
			return m, nil // Return updated model to trigger view change
		}

	case "esc", "b": // Go back to Project Stats
		m.CurrentScreen = app.ScreenProjectStats
		m.ProjectCommandsListIndex = 0
		return m, nil
	}

	// Return the updated model and any command from the paginator
	return m, paginatorCmd
}

// ViewScreenProjectCommandsList renders the list of locally saved project commands.
func ViewScreenProjectCommandsList(m app.Model, registry *project.ProjectRegistry) string {
	header := app.TitleStyle.Render("Project Commands (.nextgen/local-commands)") + "\n"

	projectCmdNames, err := getSortedProjectCommandNames(m.ProjectPath)
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

	// --- Render List ---
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
	listBuilder.WriteString("\n") // Spacer

	// Add Back button
	if isBackSelected {
		listBuilder.WriteString(app.HighlightStyle.Render("> Back") + "\n")
	} else {
		listBuilder.WriteString(app.ChoiceStyle.Render("  Back") + "\n")
	}

	leftPanel := lipgloss.NewStyle().Padding(1, 2).Width(40).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Render(listBuilder.String())

	// --- Right Pane: File Tree Preview ---
	previewContent := m.ProjectCommandPreview
	if previewContent == "" {
		previewContent = app.HelpStyle.Render("Select a command to see its file tree preview.")
	}

	// Prepend header
	folderName := filepath.Base(m.ProjectPath)
	headerPreview := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
	previewContent = headerPreview + "\n\n" + previewContent

	rightPanel := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.TerminalWidth - 40 - 8).
		Height(lipgloss.Height(leftPanel)).
		Render(previewContent)

	// --- Combine ---
	combinedPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	// --- Paginator View ---
	paginatorView := ""
	if totalCmds > p.PerPage {
		paginatorView = p.View()
	}

	footer := app.HelpStyle.Render("Use ↑/↓/←/→ to navigate, Enter to select, Esc/b to go back.")

	// Combine list, paginator, footer
	finalView := lipgloss.JoinVertical(lipgloss.Left, header, combinedPanes, "\n", paginatorView, "\n", footer)
	return finalView
}
