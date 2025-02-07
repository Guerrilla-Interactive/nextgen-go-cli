package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
// or a file (with code). The FileAction concept has been removed.
type TreeNode struct {
	Key      string     `json:"_key"`
	Type     string     `json:"_type"`
	Children []TreeNode `json:"children"`
	Code     string     `json:"code"`
	ID       string     `json:"id"`
	Name     string     `json:"name"`
}

// ExecuteJSONTemplate reads your JSON command file, creates the specified
// files/folders (applying placeholder replacements).
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

	// 3. Create all files/folders based on the template.
	for _, group := range template.FilePaths {
		basePath := filepath.Join(projectPath, group.Path)
		if err := gatherNodes(group.Nodes, basePath, projectPath, placeholders); err != nil {
			return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
		}
	}

	return nil
}

// gatherNodes creates directories or files based on the TreeNode objects
// (substituting placeholders in both names and code).
func gatherNodes(nodes []TreeNode, basePath, projectPath string, placeholders map[string]string) error {
	for _, node := range nodes {
		nodeName := replacePlaceholders(node.Name, placeholders)
		currentPath := filepath.Join(basePath, nodeName)

		// If node has children, treat it like a folder:
		if len(node.Children) > 0 {
			if err := os.MkdirAll(currentPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", currentPath, err)
			}
			if err := gatherNodes(node.Children, currentPath, projectPath, placeholders); err != nil {
				return err
			}
		} else if node.Code != "" {
			// Before writing, check if the file already exists:
			if _, err := os.Stat(currentPath); err == nil {
				// Log a notice if it already exists:
				fmt.Printf("Skipping creation of %s because it already exists.\n", currentPath)
			} else {
				// If node is a file, write its code (with placeholders replaced).
				if err := os.MkdirAll(filepath.Dir(currentPath), 0755); err != nil {
					return fmt.Errorf("failed to create parent directory for %s: %w", currentPath, err)
				}
				code := replacePlaceholders(node.Code, placeholders)
				if err := os.WriteFile(currentPath, []byte(code), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", currentPath, err)
				}
			}
		}
	}
	return nil
}

// replacePlaceholders walks through the placeholders map and replaces
// all occurrences of each placeholder key with its value.
func replacePlaceholders(content string, placeholders map[string]string) string {
	for oldVal, newVal := range placeholders {
		content = strings.ReplaceAll(content, oldVal, newVal)
	}
	return content
}

// RunJsonTemplate is a convenience function to run any JSON command file.
func RunJsonTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplate(jsonFilePath, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template: %w", err)
	}
	return nil
}

// RunJsonTemplateBytes is a convenience function to run any JSON command from in-memory bytes.
func RunJsonTemplateBytes(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template from memory: %w", err)
	}
	return nil
}

// ExecuteJSONTemplateFromMemory unmarshals the JSON template from memory and creates files/folders.
func ExecuteJSONTemplateFromMemory(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	// 1. Unmarshal the JSON into our JSONCommandTemplate struct.
	var template JSONCommandTemplate
	if err := json.Unmarshal(jsonBytes, &template); err != nil {
		return fmt.Errorf("could not parse JSON template: %w", err)
	}

	// 2. Create all files/folders based on the template.
	for _, group := range template.FilePaths {
		basePath := filepath.Join(projectPath, group.Path)
		if err := gatherNodes(group.Nodes, basePath, projectPath, placeholders); err != nil {
			return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
		}
	}

	return nil
}

// ToKebabCase is a helper to produce "hello-world" from "Hello World".
func ToKebabCase(input string) string {
	input = strings.ToLower(input)
	input = strings.ReplaceAll(input, " ", "-")
	return input
}

// ToPascalCase converts input to PascalCase, e.g. "hello world" becomes "HelloWorld".
func ToPascalCase(input string) string {
	words := splitIntoWords(input)
	for i, w := range words {
		words[i] = strings.Title(strings.ToLower(w))
	}
	return strings.Join(words, "")
}

// ToCamelCase converts input to camelCase.
func ToCamelCase(input string) string {
	pascal := ToPascalCase(input)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// ToLowercase converts input to all lowercase.
func ToLowercase(input string) string {
	return strings.ToLower(input)
}

// splitIntoWords splits a string into words based on hyphens or spaces.
func splitIntoWords(s string) []string {
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Fields(s)
}

// BuildPlaceholders creates a map of placeholder variables from a map of
// variable names to their raw values.
func BuildPlaceholders(vars map[string]string) map[string]string {
	placeholders := make(map[string]string)
	for key, value := range vars {
		placeholders["{{."+key+"}}"] = value
		placeholders["{{.PascalCase"+key+"}}"] = ToPascalCase(value)
		placeholders["{{.CamelCase"+key+"}}"] = ToCamelCase(value)
		placeholders["{{.KebabCase"+key+"}}"] = ToKebabCase(value)
		placeholders["{{.LowerCase"+key+"}}"] = strings.ToLower(value)
	}
	return placeholders
}

// BuildMultiPlaceholders builds a placeholder map that includes a main variable called "Main"
// along with additional variables.
func BuildMultiPlaceholders(mainValue string, extraVars map[string]string) map[string]string {
	placeholders := BuildPlaceholders(map[string]string{"Main": mainValue})
	for key, value := range extraVars {
		extraPlaceholders := BuildPlaceholders(map[string]string{key: value})
		for k, v := range extraPlaceholders {
			placeholders[k] = v
		}
	}
	return placeholders
}

// BuildAutoPlaceholders creates a placeholder map from the given map of variables.
func BuildAutoPlaceholders(vars map[string]string) map[string]string {
	if len(vars) == 1 {
		for _, value := range vars {
			return BuildPlaceholders(map[string]string{"Main": value})
		}
	}
	return BuildPlaceholders(vars)
}

// ----------------------------------------------------------------------------
// New helper functions to infer variable keys automatically from the JSON
// template. They scan for placeholders of the form:
//   {{.PascalCaseVariable}}, {{.CamelCaseVariable}}, etc.
// ----------------------------------------------------------------------------

// InferVariableKeys scans the input content and returns a slice of unique
// variable names (ignoring transformation prefixes).
func InferVariableKeys(content string) []string {
	// This regex matches patterns like {{.PascalCaseComponentName}},
	// {{.CamelCaseComponentName}}, etc. It captures the base variable name.
	regex := regexp.MustCompile(`{{\.(?:PascalCase|CamelCase|KebabCase|LowerCase)?([A-Za-z0-9_]+)}}`)
	matches := regex.FindAllStringSubmatch(content, -1)
	keysSet := make(map[string]struct{})
	for _, match := range matches {
		if len(match) > 1 {
			keysSet[match[1]] = struct{}{}
		}
	}
	var keys []string
	for key := range keysSet {
		keys = append(keys, key)
	}
	return keys
}

// GetTemplateVariableKeys loads the JSON template for the given command spec
// and returns the inferred variable keys.
func GetTemplateVariableKeys(spec CommandSpec) ([]string, error) {
	if spec.TemplatePath == "" {
		return nil, nil
	}
	data, err := LoadCommandTemplate(spec.TemplatePath)
	if err != nil {
		return nil, err
	}
	keys := InferVariableKeys(string(data))
	return keys, nil
}
