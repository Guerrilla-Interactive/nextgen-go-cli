package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JSONCommandTemplate is the root structure of your template JSON file.
type JSONCommandTemplate struct {
	FilePaths []FilePathGroup `json:"filePaths"`
}

// FilePathGroup describes a target path in your project plus
// an array of TreeNode objects.
type FilePathGroup struct {
	Key   string     `json:"_key"`
	Type  string     `json:"_type"`
	ID    string     `json:"id"`
	Nodes []TreeNode `json:"nodes"`
	Path  string     `json:"path"`
}

// TreeNode describes either a directory (with children)
// or a file (with code).
type TreeNode struct {
	Key      string     `json:"_key"`
	Type     string     `json:"_type"`
	Children []TreeNode `json:"children"`
	Code     string     `json:"code"`
	ID       string     `json:"id"`
	Name     string     `json:"name"`
}

// ExecuteJSONTemplate reads a JSON file describing what to generate,
// applies placeholder replacements to names and code, and creates
// the resulting files/folders within projectPath.
func ExecuteJSONTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
	// 1. Read the JSON template into memory.
	templateBytes, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return fmt.Errorf("could not read JSON template: %w", err)
	}

	// 2. Unmarshal the JSON into our JSONCommandTemplate struct.
	var template JSONCommandTemplate
	if err := json.Unmarshal(templateBytes, &template); err != nil {
		return fmt.Errorf("could not parse JSON template: %w", err)
	}

	// 3. Process each file path group in the template.
	for _, group := range template.FilePaths {
		basePath := filepath.Join(projectPath, group.Path)

		// Create the files/folders described by group.Nodes
		if err := processNodes(group.Nodes, basePath, placeholders); err != nil {
			return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
		}
	}

	return nil
}

// processNodes is a helper that recursively creates directories or
// writes files (with placeholder replacement) based on TreeNode slices.
func processNodes(nodes []TreeNode, basePath string, placeholders map[string]string) error {
	for _, node := range nodes {
		nodeName := replacePlaceholders(node.Name, placeholders)
		currentPath := filepath.Join(basePath, nodeName)

		// If the node has children, we treat it as a folder.
		if len(node.Children) > 0 {
			if err := os.MkdirAll(currentPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", currentPath, err)
			}
			if err := processNodes(node.Children, currentPath, placeholders); err != nil {
				return err
			}
		} else if node.Code != "" {
			// Otherwise, if there’s code, we treat it as a file.
			code := replacePlaceholders(node.Code, placeholders)
			if err := os.WriteFile(currentPath, []byte(code), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", currentPath, err)
			}
		}
	}
	return nil
}

// replacePlaceholders loops through the placeholders map and replaces
// all occurrences of a placeholder with its value in the provided content.
func replacePlaceholders(content string, placeholders map[string]string) string {
	for oldVal, newVal := range placeholders {
		content = strings.ReplaceAll(content, oldVal, newVal)
	}
	return content
}

// BuildNamePlaceholders is an example helper for automatically building
// typical naming placeholders (PascalCase, camelCase, kebab-case, etc.).
func BuildNamePlaceholders(rawName string) map[string]string {
	return map[string]string{
		"{example}":                    strings.ToLower(rawName),
		"{{.PascalCaseComponentName}}": toPascalCase(rawName),
		"{{.CamelCaseComponentName}}":  toCamelCase(rawName),
		"{{.KebabCaseComponentName}}":  toKebabCase(rawName),
	}
}

// Example case conversion helpers:

func toPascalCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.Title(strings.ToLower(w))
	}
	return strings.Join(words, "")
}

func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

func toKebabCase(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), "-"))
}
