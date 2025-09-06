package mainScreen

import (
	"fmt"
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
        // Top-level line; mark edited files
        displayName := name
        if child.IsFile {
            if edited, ok := commands.EditedIndexers[child.Path]; ok && edited {
                displayName += " (edited)"
            }
        }
        treeDisplay += fmt.Sprintf("%s%s\n", icon, displayName)
        // Render the children without reprinting the top-level node.
        treeDisplay += renderFileTree(child, " ", true, true)
    }

    // Log-style output: header + path + tree (no interactive options)
    header := app.TitleStyle.Render("Installation Complete! ✅")
    pathLine := app.PathStyle.Render(m.ProjectPath)
    // Add a separator line on top for clarity
    sep := strings.Repeat("─", 48)
    msg := sep + "\n" + header + "\n" + pathLine + "\n\n" + treeDisplay + "\n\n" +
        sharedScreens.Footer("Enter back", "Esc back", "Ctrl+C quit")
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
        return m, tea.Quit
    case "enter", "esc":
        // Treat as a simple log screen: Enter/Esc go back to Recent Commands
        m.CurrentScreen = app.ScreenMain
        return m, nil
    // Optional convenience keys
    case "q", "Q":
        return m, tea.Quit
    default:
        return m, nil
    }
}
