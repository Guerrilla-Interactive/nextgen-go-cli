package utils

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	// NEW: Import Lipgloss for styled output.
	"github.com/charmbracelet/lipgloss"
)

// NEW: Define route style to color filetree route lines with #444.
var routeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#444"))

// NEW: Define text style to color file names and icons with #888.
var textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))

// FileNode represents a node in the file tree.
type FileNode struct {
	Name     string
	Path     string // Full file path (only set on file nodes)
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

// BuildFileTree builds a tree structure from a slice of file paths.
func BuildFileTree(paths []string) *FileNode {
	root := &FileNode{Name: "", Children: make(map[string]*FileNode)}
	for _, fullPath := range paths {
		// Normalize the path and split by "/".
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

// IsEditedFunc is a type for a callback used to mark edited files.
type IsEditedFunc func(path string) bool

// RenderFileTree renders the file tree as a string using branch characters.
// The parameter skipSelf, if true, omits printing the current node header.
// The isEdited callback is called for file nodes; if it returns true, we append " (edited)".
func RenderFileTree(node *FileNode, prefix string, isLast bool, skipSelf bool, isEdited IsEditedFunc) string {
	var line string
	if !skipSelf && node.Name != "" {
		// Use the routeStyle to render branch characters.
		branch := routeStyle.Render("┣")
		if isLast {
			branch = routeStyle.Render("┗")
		}
		// Use 📂 for directories and 📜 for files.
		icon := "📜"
		if len(node.Children) > 0 {
			icon = "📂"
		}
		displayName := node.Name
		if node.IsFile && isEdited != nil && isEdited(node.Path) {
			displayName += " (edited)"
		}
		line = fmt.Sprintf("%s%s %s\n", prefix, branch, textStyle.Render(fmt.Sprintf("%s %s", icon, displayName)))
	}

	// Update prefix for subsequent children.
	newPrefix := prefix
	if node.Name != "" {
		if isLast {
			newPrefix += "   "
		} else {
			newPrefix += routeStyle.Render("┃") + "  "
		}
	}

	// Sort children for a consistent display.
	var names []string
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)

	result := line
	for i, name := range names {
		child := node.Children[name]
		childIsLast := i == len(names)-1
		// Always render children with skipSelf = false.
		result += RenderFileTree(child, newPrefix, childIsLast, false, isEdited)
	}
	return result
}

// NEW: RenderFileTreeWithHeader renders the file tree with a header showing the current folder.
// You can pass the current folder (or working directory) as `currentFolder`.
// It prepends a styled header before the file tree.
func RenderFileTreeWithHeader(root *FileNode, currentFolder string, isEdited IsEditedFunc) string {
	header := textStyle.Render(fmt.Sprintf("Current folder: %s", currentFolder))
	return header + "\n\n" + RenderFileTree(root, "", true, true, isEdited)
}
