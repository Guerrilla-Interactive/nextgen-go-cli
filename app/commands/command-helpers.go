package commands

import (
	"encoding/json"
	"errors"
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

// FileAction describes any follow-up actions we want to perform on other files
// such as inserting code above a particular marker or doing other transformations.
type FileAction struct {
	Key             string `json:"_key"`
	Type            string `json:"_type"`
	ActionType      string `json:"actionType"`
	Code            string `json:"code"`
	DestinationType string `json:"destinationType"` // e.g. "external", "internal"
	Marker          string `json:"marker"`
	Route           string `json:"route"`
}

// TreeNode describes either a directory (with children)
// or a file (with code). It may also contain actions that
// instruct us to patch/modify other files after creation.
type TreeNode struct {
	Key      string       `json:"_key"`
	Type     string       `json:"_type"`
	Children []TreeNode   `json:"children"`
	Code     string       `json:"code"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Actions  []FileAction `json:"actions,omitempty"`
}

// ExecuteJSONTemplate reads your JSON command file, creates the specified
// files/folders (applying placeholder replacements), and queues up
// any requested "actions" (like pasting code above a marker).
// We collect *all* actions first, then do a second pass applying them
// to avoid overwriting newly inserted code.
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

	// Collect *all* actions in a single slice so we can apply them last.
	var allActions []FileAction

	// 3. First pass: Create all files/folders (and gather actions).
	for _, group := range template.FilePaths {
		basePath := filepath.Join(projectPath, group.Path)

		actions, err := gatherNodes(group.Nodes, basePath, projectPath, placeholders)
		if err != nil {
			return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
		}

		allActions = append(allActions, actions...)
	}

	// 4. Second pass: Apply all recorded actions *after* files/folders are created.
	if err := processActions(allActions, projectPath, placeholders); err != nil {
		return fmt.Errorf("error processing file actions: %w")
	}

	return nil
}

// gatherNodes creates directories or files based on the TreeNode objects
// (substituting placeholders in both names and code).
// It returns any file actions (like "pasteAboveMarker") for a separate pass.
func gatherNodes(nodes []TreeNode, basePath, projectPath string, placeholders map[string]string) ([]FileAction, error) {
	var allActions []FileAction

	for _, node := range nodes {
		nodeName := replacePlaceholders(node.Name, placeholders)
		currentPath := filepath.Join(basePath, nodeName)

		// If node has children, treat it like a folder:
		if len(node.Children) > 0 {
			if err := os.MkdirAll(currentPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", currentPath, err)
			}
			childActions, err := gatherNodes(node.Children, currentPath, projectPath, placeholders)
			if err != nil {
				return nil, err
			}
			allActions = append(allActions, childActions...)

		} else if node.Code != "" {
			// Before writing, check if the file already exists:
			if _, err := os.Stat(currentPath); err == nil {
				// Log a notice if it already exists:
				fmt.Printf("Skipping creation of %s because it already exists.\n", currentPath)
			} else {
				// If node is a file, write its code (with placeholders replaced).
				if err := os.MkdirAll(filepath.Dir(currentPath), 0755); err != nil {
					return nil, fmt.Errorf("failed to create parent directory for %s: %w", currentPath, err)
				}
				code := replacePlaceholders(node.Code, placeholders)
				if err := os.WriteFile(currentPath, []byte(code), 0644); err != nil {
					return nil, fmt.Errorf("failed to write file %s: %w", currentPath, err)
				}
			}
		}

		// Collect any "actions" for the second pass (post-creation).
		allActions = append(allActions, node.Actions...)
	}

	return allActions, nil
}

// processActions applies modifications specified in FileAction objects,
// like inserting code above a "marker" in an existing file. Running this
// after all files are created ensures we don't overwrite inserted lines
// in a subsequent file write.
func processActions(actions []FileAction, projectPath string, placeholders map[string]string) error {
	for _, action := range actions {
		destPath := filepath.Join(projectPath, action.Route)

		fileBytes, err := os.ReadFile(destPath)
		if errors.Is(err, os.ErrNotExist) {
			// If the target file doesn't exist, create it with a marker so
			// we can insert above or below that marker.
			baseDir := filepath.Dir(destPath)
			if mkErr := os.MkdirAll(baseDir, 0755); mkErr != nil {
				return fmt.Errorf("failed to create directory for %s: %w", destPath, mkErr)
			}
			defaultContent := fmt.Sprintf("// %s\n", action.Marker)
			if initErr := os.WriteFile(destPath, []byte(defaultContent), 0644); initErr != nil {
				return fmt.Errorf("failed to initialize file at %s: %w", destPath, initErr)
			}
			// Re-read so we have the updated content
			fileBytes, err = os.ReadFile(destPath)
			if err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("failed to read file for action %q at %s: %w", action.ActionType, destPath, err)
		}

		content := string(fileBytes)
		insertCode := replacePlaceholders(action.Code, placeholders)

		switch action.ActionType {
		case "pasteAboveMarker":
			markerIndex := strings.Index(content, action.Marker)
			if markerIndex == -1 {
				// If the marker is not found, place it at the end
				markerIndex = len(content)
				content += "\n" + action.Marker + "\n"
			}
			newContent := content[:markerIndex] + insertCode + "\n" + content[markerIndex:]
			if writeErr := os.WriteFile(destPath, []byte(newContent), 0644); writeErr != nil {
				return fmt.Errorf("failed to write updated file %s: %w", destPath, writeErr)
			}

			// Additional cases could be added here:
			// case "pasteBelowMarker":
			// case "replaceMarker":
			// etc.

		default:
			return fmt.Errorf("unrecognized action type %q in node actions", action.ActionType)
		}
	}

	return nil
}

// replacePlaceholders walks through the placeholders map and replaces
// all occurrences of each placeholder key with its value. This is how
// {{.LowerCaseComponentName}}, {{.CamelCaseComponentName}}, etc.
// get turned into actual strings like "profile" or "myFile".
func replacePlaceholders(content string, placeholders map[string]string) string {
	for oldVal, newVal := range placeholders {
		content = strings.ReplaceAll(content, oldVal, newVal)
	}
	return content
}

// MakeFilenamePlaceholder is a small helper that transforms an incoming file name
// into a consistent format, e.g. all lowercase. You can adapt it if you want
// to preserve some capitalization or apply kebab-case.
func MakeFilenamePlaceholder(fileBaseName string) string {
	return strings.ToLower(fileBaseName)
}

// RunJsonTemplate is a convenience function to run any JSON command file,
// such as "page-and-archive.json" or "sample-command.json".
func RunJsonTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplate(jsonFilePath, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template: %w", err)
	}
	return nil
}

// ToKebabCase is a helper to produce "hello-world" from "Hello World".
func ToKebabCase(input string) string {
	input = strings.ToLower(input)
	input = strings.ReplaceAll(input, " ", "-")
	return input
}

// ToPascalCase is a helper to produce "HelloWorld" from "hello-world".
func ToPascalCase(input string) string {
	words := splitIntoWords(input)
	for i, w := range words {
		words[i] = strings.Title(strings.ToLower(w))
	}
	return strings.Join(words, "")
}

// ToCamelCase is a helper that converts "HelloWorld" → "helloWorld".
func ToCamelCase(input string) string {
	pascal := ToPascalCase(input)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// ToLowercase is a helper to produce lowercase versions of a string only.
func ToLowercase(input string) string {
	return strings.ToLower(input)
}

// splitIntoWords splits on hyphens or spaces, used internally by case-converters.
func splitIntoWords(s string) []string {
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Fields(s)
}

// BuildNamePlaceholders can build a map of placeholders (like {{.CamelCaseComponentName}})
// from a single raw name. That way, we can do things like insert "myFile" or "MyFile" or
// "my-file" automatically in code snippets based on user choices.
func BuildNamePlaceholders(rawName string) map[string]string {
	return map[string]string{
		"{example}":                    strings.ToLower(rawName),
		"{{.PascalCaseComponentName}}": ToPascalCase(rawName),
		"{{.CamelCaseComponentName}}":  ToCamelCase(rawName),
		"{{.KebabCaseComponentName}}":  ToKebabCase(rawName),
		"{{.LowerCaseComponentName}}":  strings.ToLower(rawName),
	}
}

func RunJsonTemplateBytes(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template from memory: %w", err)
	}
	return nil
}

func ExecuteJSONTemplateFromMemory(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	// 1. Unmarshal the JSON into our JSONCommandTemplate struct.
	var template JSONCommandTemplate
	if err := json.Unmarshal(jsonBytes, &template); err != nil {
		return fmt.Errorf("could not parse JSON template: %w", err)
	}

	// 2. Gather nodes, create files, etc. same as your existing code...
	var allActions []FileAction
	for _, group := range template.FilePaths {
		basePath := filepath.Join(projectPath, group.Path)
		actions, err := gatherNodes(group.Nodes, basePath, projectPath, placeholders)
		if err != nil {
			return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
		}
		allActions = append(allActions, actions...)
	}

	// 3. Second pass: apply the file actions
	if err := processActions(allActions, projectPath, placeholders); err != nil {
		return fmt.Errorf("error processing file actions: %w", err)
	}

	return nil
}
