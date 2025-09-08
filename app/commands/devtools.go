// commands/devtools.go
package commands

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils"
	"github.com/atotto/clipboard"
)

//
// Devtools: preview, metadata, and template inspection helpers
//

// GeneratePreviewFileTree generates a string representation of the file tree
// that *would* be created by a given command, without actually writing files.
func GeneratePreviewFileTree(cmdName string, placeholders map[string]string, projectPath string) (string, error) {
	// Load command spec or embedded template by path.
	spec := GetCommandSpec(cmdName)

	var data []byte
	var err error
	if spec.TemplatePath != "" {
		data, err = LoadCommandTemplate(spec.TemplatePath)
		if err != nil {
			return "", fmt.Errorf("failed to load template: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(cmdName), ".json") {
		// Allow previewing embedded templates by full path
		data, err = LoadCommandTemplate(cmdName)
		if err != nil {
			return "", fmt.Errorf("failed to load template by path: %w", err)
		}
	} else {
		return "", fmt.Errorf("command %q has no template", cmdName)
	}

	// Unmarshal into the JSONCommandTemplate structure using original data.
	var tmpl JSONCommandTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return "", fmt.Errorf("failed to parse template JSON: %w", err)
	}

	// Collect file paths that would be created.
	var filePaths []string
	for _, group := range tmpl.FilePaths {
		// Calculate the base path for the group, applying placeholders HERE.
		base := filepath.Join(projectPath, replacePlaceholders(group.Path, placeholders))
		// Recursively collect file paths from the tree nodes, applying placeholders inside.
		var collectFiles func(nodes []TreeNode, currPath string) []string
		collectFiles = func(nodes []TreeNode, currPath string) []string {
			var paths []string
			for _, n := range nodes {
				// Apply placeholders to name HERE.
				name := replacePlaceholders(n.Name, placeholders)
				fullPath := filepath.Join(currPath, name)
				if len(n.Children) > 0 {
					paths = append(paths, collectFiles(n.Children, fullPath)...)
				} else {
					paths = append(paths, fullPath)
				}
			}
			return paths
		}
		filePaths = append(filePaths, collectFiles(group.Nodes, base)...)
	}

	// Convert filePaths to relative paths so that the preview only shows files from the launch folder.
	var relPaths []string
	for _, f := range filePaths {
		if rel, err := filepath.Rel(projectPath, f); err == nil {
			relPaths = append(relPaths, rel)
		} else {
			relPaths = append(relPaths, f)
		}
	}

	// Build the file tree using the shared utils package.
	treeRoot := utils.BuildFileTree(relPaths)
	preview := utils.RenderFileTree(treeRoot, "", true, false, func(path string) bool {
		// Highlight edited indexers if known
		if edited, ok := EditedIndexers[path]; ok && edited {
			return true
		}
		return false
	})
	return preview, nil
}

// GeneratePreviewFileTreeFromClipboard reads the clipboard content (assumed to be a JSON
// template), applies the provided placeholders, and returns the preview file tree.
func GeneratePreviewFileTreeFromClipboard(placeholders map[string]string, projectPath string) (string, error) {
	// Retry a couple of times in case clipboard just changed
	var clipboardContent string
	var err error
	for i := 0; i < 2; i++ {
		clipboardContent, err = clipboard.ReadAll()
		if err == nil && strings.TrimSpace(clipboardContent) != "" {
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard: %w", err)
	}

	var tmpl JSONCommandTemplate
	if err := json.Unmarshal([]byte(clipboardContent), &tmpl); err != nil {
		return "", fmt.Errorf("failed to parse clipboard JSON: %w", err)
	}

	// Collect file paths that would be created.
	var filePaths []string
	for _, group := range tmpl.FilePaths {
		// Apply placeholders HERE
		base := filepath.Join(projectPath, replacePlaceholders(group.Path, placeholders))
		var collectFiles func(nodes []TreeNode, currPath string) []string
		collectFiles = func(nodes []TreeNode, currPath string) []string {
			var paths []string
			for _, n := range nodes {
				// Apply placeholders HERE
				name := replacePlaceholders(n.Name, placeholders)
				fullPath := filepath.Join(currPath, name)
				if len(n.Children) > 0 {
					paths = append(paths, collectFiles(n.Children, fullPath)...)
				} else {
					paths = append(paths, fullPath)
				}
			}
			return paths
		}
		filePaths = append(filePaths, collectFiles(group.Nodes, base)...)
	}

	// Convert filePaths to relative paths.
	var relPaths []string
	for _, f := range filePaths {
		if rel, err := filepath.Rel(projectPath, f); err == nil {
			relPaths = append(relPaths, rel)
		} else {
			relPaths = append(relPaths, f)
		}
	}

	// Build the file tree using the shared utils package.
	treeRoot := utils.BuildFileTree(relPaths)
	preview := utils.RenderFileTree(treeRoot, "", true, false, func(path string) bool {
		if edited, ok := EditedIndexers[path]; ok && edited {
			return true
		}
		return false
	})
	return preview, nil
}

// ExtractVariablesFromClipboard reads the clipboard content and extracts
// variable keys if it contains valid JSON template content.
//
// It supports two modes:
//  1. If clipboard contains valid JSON template, it parses each node.Code and extracts placeholders.
//  2. Otherwise, it scans the raw text for placeholder patterns.
//
// Returned keys are de-duplicated and sorted in insertion order.
func ExtractVariablesFromClipboard() ([]string, error) {
	clipboardContent, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %w", err)
	}

	// First, try to parse it as JSON to check if it's a valid template
	var tmpl JSONCommandTemplate
	if err := json.Unmarshal([]byte(clipboardContent), &tmpl); err != nil {
		// Not valid JSON, just extract variable keys from the text
		return InferVariableKeys(clipboardContent), nil
	}

	// Valid JSON template - extract variables from all code blocks
	vars := make(map[string]struct{})
	for _, group := range tmpl.FilePaths {
		var processNodes func(nodes []TreeNode)
		processNodes = func(nodes []TreeNode) {
			for _, node := range nodes {
				if node.Code != "" {
					// Extract variables from code content
					for _, key := range InferVariableKeys(node.Code) {
						vars[key] = struct{}{}
					}
				}
				// Process children recursively
				if len(node.Children) > 0 {
					processNodes(node.Children)
				}
			}
		}
		processNodes(group.Nodes)
	}

	// Convert map to slice
	var result []string
	for key := range vars {
		result = append(result, key)
	}
	return result, nil
}

// GeneratePreviewFileTreeFromBytes generates a file tree preview from template bytes.
// Similar to GeneratePreviewFileTree but takes byte slice instead of command name.
func GeneratePreviewFileTreeFromBytes(templateBytes []byte, placeholders map[string]string, projectPath string) (string, error) {
	// Unmarshal into the JSONCommandTemplate structure.
	var tmpl JSONCommandTemplate
	if err := json.Unmarshal(templateBytes, &tmpl); err != nil {
		return "", fmt.Errorf("failed to parse template JSON from bytes: %w", err)
	}

	// Collect file paths that would be created.
	var filePaths []string
	for _, group := range tmpl.FilePaths {
		base := filepath.Join(projectPath, replacePlaceholders(group.Path, placeholders))
		var collectFiles func(nodes []TreeNode, currPath string) []string
		collectFiles = func(nodes []TreeNode, currPath string) []string {
			var paths []string
			for _, n := range nodes {
				name := replacePlaceholders(n.Name, placeholders)
				fullPath := filepath.Join(currPath, name)
				if len(n.Children) > 0 {
					paths = append(paths, collectFiles(n.Children, fullPath)...)
				} else {
					paths = append(paths, fullPath)
				}
			}
			return paths
		}
		filePaths = append(filePaths, collectFiles(group.Nodes, base)...)
	}

	// Convert filePaths to relative paths.
	var relPaths []string
	for _, f := range filePaths {
		if rel, err := filepath.Rel(projectPath, f); err == nil {
			relPaths = append(relPaths, rel)
		} else {
			relPaths = append(relPaths, f)
		}
	}

	// Build the file tree using the shared utils package.
	treeRoot := utils.BuildFileTree(relPaths)
	preview := utils.RenderFileTree(treeRoot, "", true, false, func(path string) bool {
		// Preview doesn't know about edited indexers in this context
		return false
	})
	return preview, nil
}

//
// Template introspection helpers
//

// GetCommandVariableDescriptions attempts to extract human-friendly descriptions
// for variables defined by a command template. It supports two schema shapes:
// 1) variables: { "VarName": { "description": "..." }, ... }
// 2) args: [ { "name": "VarName", "message": "..." | "description": "..." }, ... ]
func GetCommandVariableDescriptions(cmdName, projectPath string, registry *project.ProjectRegistry) (map[string]string, error) {
	descs := map[string]string{}
	// Try to load the template bytes via common resolution
	b, _, err := LoadTemplateBytesForName(cmdName, projectPath, registry)
	if err != nil {
		// If cmdName is an embedded template path, try that directly
		if data, readErr := LoadCommandTemplate(cmdName); readErr == nil {
			b = data
		} else {
			return descs, nil
		}
	}
	var obj map[string]any
	if jerr := json.Unmarshal(b, &obj); jerr != nil {
		return descs, nil
	}
	// Case 1: variables object
	if raw, ok := obj["variables"]; ok {
		if m, ok2 := raw.(map[string]any); ok2 {
			for k, v := range m {
				if inner, ok3 := v.(map[string]any); ok3 {
					if d, ok4 := inner["description"]; ok4 {
						if s, ok5 := d.(string); ok5 && strings.TrimSpace(s) != "" {
							descs[k] = s
						}
					}
				}
			}
		}
	}
	// Case 2: args array (use description or message)
	if raw, ok := obj["args"]; ok {
		if arr, ok2 := raw.([]any); ok2 {
			for _, it := range arr {
				if m, ok3 := it.(map[string]any); ok3 {
					nameVal, _ := m["name"].(string)
					if strings.TrimSpace(nameVal) == "" {
						continue
					}
					var d string
					if dv, ok4 := m["description"]; ok4 {
						if s, ok5 := dv.(string); ok5 {
							d = s
						}
					}
					if strings.TrimSpace(d) == "" {
						if mv, ok6 := m["message"]; ok6 {
							if s, ok7 := mv.(string); ok7 {
								d = s
							}
						}
					}
					if strings.TrimSpace(d) != "" {
						descs[nameVal] = d
					}
				}
			}
		}
	}
	return descs, nil
}

// GetCommandVariableTitles extracts display titles for variables from the template, if provided.
// Supports variables.<Var>.title and args[].title (falls back to args[].label if present).
func GetCommandVariableTitles(cmdName, projectPath string, registry *project.ProjectRegistry) (map[string]string, error) {
	titles := map[string]string{}
	b, _, err := LoadTemplateBytesForName(cmdName, projectPath, registry)
	if err != nil {
		if data, readErr := LoadCommandTemplate(cmdName); readErr == nil {
			b = data
		} else {
			return titles, nil
		}
	}
	var obj map[string]any
	if jerr := json.Unmarshal(b, &obj); jerr != nil {
		return titles, nil
	}
	// 1) variables object
	if raw, ok := obj["variables"]; ok {
		if m, ok2 := raw.(map[string]any); ok2 {
			for k, v := range m {
				if inner, ok3 := v.(map[string]any); ok3 {
					if t, ok4 := inner["title"]; ok4 {
						if s, ok5 := t.(string); ok5 && strings.TrimSpace(s) != "" {
							titles[k] = s
						}
					}
				}
			}
		}
	}
	// 2) args array: title or label fields
	if raw, ok := obj["args"]; ok {
		if arr, ok2 := raw.([]any); ok2 {
			for _, it := range arr {
				if m, ok3 := it.(map[string]any); ok3 {
					nameVal, _ := m["name"].(string)
					if strings.TrimSpace(nameVal) == "" {
						continue
					}
					var t string
					if tv, ok4 := m["title"]; ok4 {
						if s, ok5 := tv.(string); ok5 {
							t = s
						}
					}
					if strings.TrimSpace(t) == "" {
						if lv, ok6 := m["label"]; ok6 {
							if s, ok7 := lv.(string); ok7 {
								t = s
							}
						}
					}
					if strings.TrimSpace(t) != "" {
						titles[nameVal] = t
					}
				}
			}
		}
	}
	return titles, nil
}

// GetCommandVariablePriorities extracts numeric priorities for variables from the template.
// Supports variables.<Var>.priority (number) and args[].priority (number).
// Lower values indicate earlier prompting.
func GetCommandVariablePriorities(cmdName, projectPath string, registry *project.ProjectRegistry) (map[string]int, error) {
	res := map[string]int{}
	b, _, err := LoadTemplateBytesForName(cmdName, projectPath, registry)
	if err != nil {
		if data, readErr := LoadCommandTemplate(cmdName); readErr == nil {
			b = data
		} else {
			return res, nil
		}
	}
	var obj map[string]any
	if jerr := json.Unmarshal(b, &obj); jerr != nil {
		return res, nil
	}
	// variables object
	if raw, ok := obj["variables"]; ok {
		if m, ok2 := raw.(map[string]any); ok2 {
			for k, v := range m {
				if inner, ok3 := v.(map[string]any); ok3 {
					if pv, ok4 := inner["priority"]; ok4 {
						switch t := pv.(type) {
						case float64:
							res[k] = int(t)
						case int:
							res[k] = t
						case json.Number:
							if iv, e := t.Int64(); e == nil {
								res[k] = int(iv)
							}
						case string:
							if n, e := json.Number(t).Int64(); e == nil {
								res[k] = int(n)
							}
						}
					}
				}
			}
		}
	}
	// args array
	if raw, ok := obj["args"]; ok {
		if arr, ok2 := raw.([]any); ok2 {
			for _, it := range arr {
				if m, ok3 := it.(map[string]any); ok3 {
					nameVal, _ := m["name"].(string)
					if strings.TrimSpace(nameVal) == "" {
						continue
					}
					if pv, ok4 := m["priority"]; ok4 {
						switch t := pv.(type) {
						case float64:
							res[nameVal] = int(t)
						case int:
							res[nameVal] = t
						case json.Number:
							if iv, e := t.Int64(); e == nil {
								res[nameVal] = int(iv)
							}
						case string:
							if n, e := json.Number(t).Int64(); e == nil {
								res[nameVal] = int(n)
							}
						}
					}
				}
			}
		}
	}
	return res, nil
}

// GetCommandVariableExamples extracts example values per variable from the template.
// Supports variables.<Var>.examples: []string and args[].examples: []string
func GetCommandVariableExamples(cmdName, projectPath string, registry *project.ProjectRegistry) (map[string][]string, error) {
	out := map[string][]string{}
	b, _, err := LoadTemplateBytesForName(cmdName, projectPath, registry)
	if err != nil {
		if data, readErr := LoadCommandTemplate(cmdName); readErr == nil {
			b = data
		} else {
			return out, nil
		}
	}
	var obj map[string]any
	if jerr := json.Unmarshal(b, &obj); jerr != nil {
		return out, nil
	}
	// variables object
	if raw, ok := obj["variables"]; ok {
		if m, ok2 := raw.(map[string]any); ok2 {
			for k, v := range m {
				if inner, ok3 := v.(map[string]any); ok3 {
					if exv, ok4 := inner["examples"]; ok4 {
						if arr, ok5 := exv.([]any); ok5 {
							var coll []string
							for _, it := range arr {
								if s, ok6 := it.(string); ok6 && strings.TrimSpace(s) != "" {
									coll = append(coll, s)
								}
							}
							if len(coll) > 0 {
								out[k] = coll
							}
						}
					}
				}
			}
		}
	}
	// args array
	if raw, ok := obj["args"]; ok {
		if arr, ok2 := raw.([]any); ok2 {
			for _, it := range arr {
				if m, ok3 := it.(map[string]any); ok3 {
					nameVal, _ := m["name"].(string)
					if strings.TrimSpace(nameVal) == "" {
						continue
					}
					if exv, ok4 := m["examples"]; ok4 {
						if arr2, ok5 := exv.([]any); ok5 {
							var coll []string
							for _, it2 := range arr2 {
								if s, ok6 := it2.(string); ok6 && strings.TrimSpace(s) != "" {
									coll = append(coll, s)
								}
							}
							if len(coll) > 0 {
								out[nameVal] = coll
							}
						}
					}
				}
			}
		}
	}
	return out, nil
}

// Note: InferVariableKeys is defined in command-registry.go and reused here.Å
