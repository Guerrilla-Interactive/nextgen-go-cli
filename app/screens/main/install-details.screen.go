package mainScreen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileNode represents a node in the file tree.
type FileNode struct {
	Name     string
	Path     string // New field: full file path (for files)
	IsFile   bool
	Children map[string]*FileNode
}

// addChild adds (or retrieves) a child node.
func (n *FileNode) addChild(name string, isFile bool) *FileNode {
	if n.Children == nil {
		n.Children = make(map[string]*FileNode)
	}
	if child, ok := n.Children[name]; ok {
		return child
	}
	child := &FileNode{
		Name:   name,
		IsFile: isFile,
	}
	n.Children[name] = child
	return child
}

// buildFileTree builds a tree structure from a slice of file paths.
func buildFileTree(paths []string) *FileNode {
	root := &FileNode{Name: "", Children: make(map[string]*FileNode)}
	for _, fullPath := range paths {
		// Normalize path separators.
		normPath := filepath.ToSlash(fullPath)
		parts := strings.Split(normPath, "/")
		current := root
		for i, part := range parts {
			isFile := (i == len(parts)-1)
			child := current.addChild(part, isFile)
			if isFile {
				// Store the full path on file nodes.
				child.Path = fullPath
			}
			current = child
		}
	}
	return root
}

// renderFileTree returns a string representing the tree using branch characters.
// The new parameter "skipSelf" allows the caller to omit printing the current node's header.
func renderFileTree(node *FileNode, prefix string, isLast bool, skipSelf bool) string {
	var line string
	if !skipSelf && node.Name != "" {
		branch := "┣"
		if isLast {
			branch = "┗"
		}
		// Use 📂 for directories and 📜 for files.
		icon := "📜"
		if len(node.Children) > 0 {
			icon = "📂"
		}
		displayName := node.Name
		// If this is a file and is marked as edited, append " (edited)".
		if node.IsFile {
			if edited, ok := commands.EditedIndexers[node.Path]; ok && edited {
				displayName += " (edited)"
			}
		}
		line = fmt.Sprintf("%s%s %s %s\n", prefix, branch, icon, displayName)
	}

	// Update prefix for subsequent children.
	newPrefix := prefix
	if node.Name != "" {
		if isLast {
			newPrefix += "   "
		} else {
			newPrefix += "┃  "
		}
	}

	// Sort children alphabetically.
	var names []string
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	result := line
	for i, name := range names {
		child := node.Children[name]
		childIsLast := i == len(names)-1
		// Always print children with skipSelf = false.
		result += renderFileTree(child, newPrefix, childIsLast, false)
	}
	return result
}

// ViewInstallDetailsScreen builds and returns the installation details screen,
// including a tree view of the created file paths.
func ViewInstallDetailsScreen(m app.Model) string {
	// Use the created files recorded during execution.
	var relPaths []string
	for _, fullPath := range commands.CreatedFiles {
		// Convert full paths to relative paths based on the project root, if possible.
		if m.ProjectPath != "" {
			if rel, err := filepath.Rel(m.ProjectPath, fullPath); err == nil {
				relPaths = append(relPaths, rel)
				continue
			}
		}
		relPaths = append(relPaths, fullPath)
	}

	// Build a tree from the relative paths.
	treeRoot := buildFileTree(relPaths)

	// Render the top-level nodes.
	var treeDisplay string
	// Sort top-level names.
	var topLevel []string
	for name := range treeRoot.Children {
		topLevel = append(topLevel, name)
	}
	sort.Strings(topLevel)
	for _, name := range topLevel {
		child := treeRoot.Children[name]
		// Use 📦 for directories and 📜 for files.
		icon := "📜"
		if len(child.Children) > 0 {
			icon = "📦"
		}
		// Print the top-level folder header.
		treeDisplay += fmt.Sprintf("%s%s\n", icon, name)
		// Render the children without reprinting the top-level node.
		treeDisplay += renderFileTree(child, " ", true, true)
	}

	// Header section using TitleStyle and PathStyle.

	header := lipgloss.JoinVertical(lipgloss.Center,
		app.TitleStyle.Render("Installation Complete! ✅"),
	)
	pathLine := app.PathStyle.Render(m.ProjectPath)

	// Build the file tree container using Lipgloss for a refined look.
	treeContainer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1).
		Render(treeDisplay)

	// Options panel as a navigable list with three options.
	optionsList := []string{"[Recent Commands]", "[Run Command Again]", "[Exit]"}
	var optionsText string
	for i, option := range optionsList {
		if i == m.InstallDetailsSelectedOption {
			optionsText += app.HighlightStyle.Render(option)
		} else {
			optionsText += app.HelpStyle.Render(option)
		}
		if i < len(optionsList)-1 {
			optionsText += "    "
		}
	}
	options := optionsText

	// Combine header, file tree and options; then append the help notice.
	msg := header + "\n" + pathLine + "\n" + treeContainer + "\n\n" + options + "\n\n" +
		app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)")
	finalView := sharedScreens.BaseContainer(msg)
	if m.TerminalWidth > 0 && m.TerminalHeight > 0 {
		return lipgloss.Place(m.TerminalWidth, m.TerminalHeight, lipgloss.Left, lipgloss.Bottom, finalView)
	}
	return finalView
}

// UpdateInstallDetailsScreen handles key input for the Install Details screen.
func UpdateInstallDetailsScreen(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		// Quit the app when ctrl+c is pressed.
		os.Exit(0)
	case "left", "h", "up", "k":
		// Cycle left: decrement selection mod 3.
		m.InstallDetailsSelectedOption = (m.InstallDetailsSelectedOption + 3 - 1) % 3
		return m, nil
	case "right", "l", "down", "j":
		// Cycle right: increment selection mod 3.
		m.InstallDetailsSelectedOption = (m.InstallDetailsSelectedOption + 1) % 3
		return m, nil
	case "enter":
		if m.InstallDetailsSelectedOption == 0 {
			// Recent Commands: Go back to that screen.
			m.CurrentScreen = app.ScreenMain
			return m, nil
		} else if m.InstallDetailsSelectedOption == 1 {
			// Run Command Again: Navigate to the filename prompt screen and clear the input.
			m.CurrentScreen = app.ScreenFilenamePrompt
			m.TempFilename = ""
			m.FileTreePreview = ""
			return m, nil
		} else if m.InstallDetailsSelectedOption == 2 {
			// Exit.
			return m, tea.Quit
		}
		return m, nil
	// Direct key shortcuts.
	case "b", "B":
		// Shortcut to go back to Recent Commands.
		m.CurrentScreen = app.ScreenMain
		return m, nil
	case "r", "R":
		// Shortcut to run command again: navigate to the filename prompt screen.
		m.CurrentScreen = app.ScreenFilenamePrompt
		m.TempFilename = ""
		m.FileTreePreview = ""
		return m, nil
	case "q", "Q", "e", "E":
		// Shortcut to exit.
		return m, tea.Quit
	default:
		// For any other key, log (if desired) and return the current model.
		// For debugging: fmt.Println("Unhandled key:", msg.String())
		return m, nil
	}
	return m, nil
}
