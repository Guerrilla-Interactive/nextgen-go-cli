package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils"
	"github.com/atotto/clipboard"
)

// -----------------------------------------------------------------------------
// [NEW] Smart snippet merge helpers
// -----------------------------------------------------------------------------

// Global regex patterns for snippet markers.
var (
	startMarkerRegex = regexp.MustCompile(`^\s*//\s*START\s+OF\s+(.+)$`)
	endMarkerRegex   = regexp.MustCompile(`^\s*//\s*END\s+OF\s+(.+)$`)
	addMarkerRegex   = regexp.MustCompile(`^\s*//\s*ADD\s+(.+?)\s+(BELOW|ABOVE)\s*$`)
)

// removeSnippetMarkers removes the marker lines (START/END) from the
// provided content while keeping the snippet's code intact. This is used
// when creating a new file.
func removeSnippetMarkers(content string) string {
	var output []string
	lines := strings.Split(content, "\n")
	collecting := false
	for _, line := range lines {
		if startMarkerRegex.MatchString(line) {
			// Do not output the start marker; begin collecting snippet code.
			collecting = true
			continue
		}
		if collecting && endMarkerRegex.MatchString(line) {
			// End marker encountered; stop collecting.
			collecting = false
			continue
		}
		// Always output the line (whether in snippet or normal code)
		output = append(output, line)
	}
	return strings.Join(output, "\n")
}

// extractSnippets scans the content for snippet groups delimited by
// "// START OF ..." and "// END OF ..." markers. It returns a map where
// each key (e.g. "VALUE 1") maps to the snippet code found in between.
func extractSnippets(content string) (map[string]string, error) {
	snippets := make(map[string][]string)
	lines := strings.Split(content, "\n")
	var currentKey string
	collecting := false
	for _, line := range lines {
		if m := startMarkerRegex.FindStringSubmatch(line); m != nil {
			currentKey = strings.TrimSpace(m[1])
			collecting = true
			snippets[currentKey] = []string{}
			continue
		}
		if collecting {
			if m := endMarkerRegex.FindStringSubmatch(line); m != nil {
				collecting = false
				currentKey = ""
				continue
			}
			snippets[currentKey] = append(snippets[currentKey], line)
		}
	}
	result := make(map[string]string)
	for key, lines := range snippets {
		// Join the snippet's lines and trim any extra whitespace.
		result[key] = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return result, nil
}

// smartMerge takes an existing file's content and the new template content,
// extracts snippet(s) from the new content, and then looks for any insertion
// markers (like "// ADD VALUE 1 BELOW") in the existing file. When found, the
// snippet code is inserted (either above or below the marker).
func smartMerge(existingContent, templateContent string) (string, error) {
	// First, extract any snippet groups defined in the new template.
	snippetMap, err := extractSnippets(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to extract snippets: %w", err)
	}

	lines := strings.Split(existingContent, "\n")
	var mergedLines []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// Check if this line is an insertion marker.
		if matches := addMarkerRegex.FindStringSubmatch(line); matches != nil {
			key := strings.TrimSpace(matches[1])    // e.g. "VALUE 1"
			position := strings.ToUpper(matches[2]) // either "BELOW" or "ABOVE"
			// If the new snippet was defined in the template then add it.
			if snippet, ok := snippetMap[key]; ok && snippet != "" {
				snippetLines := strings.Split(snippet, "\n")
				if position == "BELOW" {
					mergedLines = append(mergedLines, line)
					// (A simple check is performed here to try and avoid duplicate insertions.)
					if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == strings.TrimSpace(snippetLines[0]) {
						// Assume the snippet has already been inserted.
					} else {
						for _, s := range snippetLines {
							mergedLines = append(mergedLines, s)
						}
					}
					continue // skip adding the marker again
				} else if position == "ABOVE" {
					// For ABOVE, insert the snippet lines just before the marker.
					if len(mergedLines) > 0 && strings.TrimSpace(mergedLines[len(mergedLines)-1]) == strings.TrimSpace(snippetLines[len(snippetLines)-1]) {
						mergedLines = append(mergedLines, line)
					} else {
						for _, s := range snippetLines {
							mergedLines = append(mergedLines, s)
						}
						mergedLines = append(mergedLines, line)
					}
					continue
				}
			}
		}
		mergedLines = append(mergedLines, line)
	}
	return strings.Join(mergedLines, "\n"), nil
}

// -----------------------------------------------------------------------------
// (Existing functions below unchanged...)
// -----------------------------------------------------------------------------

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
	Key       string     `json:"_key"`
	Type      string     `json:"_type"`
	Children  []TreeNode `json:"children"`
	Code      string     `json:"code"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	IsIndexer bool       `json:"isIndexer"` // even if false, we'll override if we see the marker in the code
}

// Global variable to record created file paths.
var CreatedFiles []string

// EditedIndexers holds file paths that are indexers and have been edited.
var EditedIndexers = make(map[string]bool)

// MarkEditedIndexer marks the given file path as an edited indexer.
func MarkEditedIndexer(path string) {
	EditedIndexers[path] = true
}

// RecordCreatedFile appends a created file path to the global CreatedFiles list.
func RecordCreatedFile(path string) {
	// Only add if not already present.
	for _, p := range CreatedFiles {
		if p == path {
			return
		}
	}
	CreatedFiles = append(CreatedFiles, path)
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

// Updated gatherNodes creates directories or files based on the TreeNode objects.
// If a file already exists then it smartly "merges" new snippet content into it,
// by searching for insertion markers like "// ADD VALUE 1 BELOW". If the file does
// not exist it simply writes the new file (after removing the snippet start/end markers).
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
			// Ensure that the parent directory exists:
			if err := os.MkdirAll(filepath.Dir(currentPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", currentPath, err)
			}
			code := replacePlaceholders(node.Code, placeholders)

			// Check for indexer marker in the code and register file as an indexer file.
			isIndexer := node.IsIndexer
			if !isIndexer && strings.Contains(code, "// THIS IS AN INDEXER FILE") {
				isIndexer = true
				fmt.Printf("ℹ️  Detected indexer marker in file %s, registering as an indexer file.\n", currentPath)
			}

			// If file already exists then we introduce smart merge behavior.
			if _, err := os.Stat(currentPath); err == nil {
				existingContentBytes, readErr := os.ReadFile(currentPath)
				if readErr != nil {
					return fmt.Errorf("failed to read existing file %s: %w", currentPath, readErr)
				}
				mergedContent, mergeErr := smartMerge(string(existingContentBytes), code)
				if mergeErr != nil {
					return fmt.Errorf("failed to merge file %s: %w", currentPath, mergeErr)
				}
				if err := os.WriteFile(currentPath, []byte(mergedContent), 0644); err != nil {
					return fmt.Errorf("failed to write merged file %s: %w", currentPath, err)
				}
				fmt.Printf("✓ Merged updates into existing file %s.\n", currentPath)
				// If this is an indexer, mark it as edited using a relative path if possible.
				if isIndexer {
					if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
						MarkEditedIndexer(rel)
					} else {
						MarkEditedIndexer(currentPath)
					}
				}
				// Record the file (including indexer files) using a relative path if possible.
				if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
					RecordCreatedFile(rel)
				} else {
					RecordCreatedFile(currentPath)
				}
			} else {
				// New file: remove the snippet start/end markers (but keep the "ADD VALUE" markers).
				newContent := removeSnippetMarkers(code)
				if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil {
					return fmt.Errorf("failed to write file %s: %w", currentPath, err)
				}
				// Record the created file using a relative path if possible.
				if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
					RecordCreatedFile(rel)
				} else {
					RecordCreatedFile(currentPath)
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
		// Without spaces.
		placeholders[fmt.Sprintf("{{.%s}}", key)] = value
		placeholders[fmt.Sprintf("{{.PascalCase%s}}", key)] = ToPascalCase(value)
		placeholders[fmt.Sprintf("{{.CamelCase%s}}", key)] = ToCamelCase(value)
		placeholders[fmt.Sprintf("{{.KebabCase%s}}", key)] = ToKebabCase(value)
		placeholders[fmt.Sprintf("{{.LowerCase%s}}", key)] = strings.ToLower(value)
		placeholders[fmt.Sprintf("{{.UpperCase%s}}", key)] = strings.ToUpper(value)

		// With extra spaces (in case tokens include spaces).
		placeholders[fmt.Sprintf("{{ .%s }}", key)] = value
		placeholders[fmt.Sprintf("{{ .PascalCase%s }}", key)] = ToPascalCase(value)
		placeholders[fmt.Sprintf("{{ .CamelCase%s }}", key)] = ToCamelCase(value)
		placeholders[fmt.Sprintf("{{ .KebabCase%s }}", key)] = ToKebabCase(value)
		placeholders[fmt.Sprintf("{{ .LowerCase%s }}", key)] = strings.ToLower(value)
		placeholders[fmt.Sprintf("{{ .UpperCase%s }}", key)] = strings.ToUpper(value)
	}
	return placeholders
}

// BuildMultiPlaceholders builds a placeholder map that includes a main variable called "Main"
// along with additional variables.
func BuildMultiPlaceholders(mainValue string, extraVars map[string]string) map[string]string {
	// Create base placeholders with the main value
	placeholders := BuildPlaceholders(map[string]string{"Main": mainValue})

	// Add each extra variable with its own transformations
	for key, value := range extraVars {
		extraPlaceholders := BuildPlaceholders(map[string]string{key: value})
		for k, v := range extraPlaceholders {
			placeholders[k] = v
		}
	}

	return placeholders
}

// BuildAutoPlaceholders builds a placeholder map from the given map of variables.
func BuildAutoPlaceholders(vars map[string]string) map[string]string {
	if len(vars) == 1 {
		for k, value := range vars {
			// If the key is "Main", then apply the default behavior.
			if k == "Main" {
				return BuildPlaceholders(map[string]string{"Main": value})
			}
			// Otherwise, preserve the provided key.
			return BuildPlaceholders(vars)
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
	// {{.CamelCaseComponentName}}, etc. It now also includes "UpperCase".
	regex := regexp.MustCompile(`{{\.(?:PascalCase|CamelCase|KebabCase|LowerCase|UpperCase)?([A-Za-z0-9_]+)}}`)
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
		// For commands like "paste from clipboard", we don't have an embedded template.
		// So, try to read the clipboard and infer variable keys from its content.
		if strings.ToLower(spec.Name) == "paste from clipboard" {
			clipboardContent, err := clipboard.ReadAll()
			if err != nil {
				// Fallback if reading the clipboard fails.
				return []string{"Filename"}, nil
			}
			keys := InferVariableKeys(clipboardContent)
			if len(keys) == 0 {
				// If no keys found, return a default key.
				return []string{"Filename"}, nil
			}
			return keys, nil
		}
		return nil, nil
	}
	data, err := LoadCommandTemplate(spec.TemplatePath)
	if err != nil {
		return nil, err
	}
	keys := InferVariableKeys(string(data))
	return keys, nil
}

// GeneratePreviewFileTree generates a file tree preview (as a string) for the given command.
// It loads the command's JSON template, applies the provided placeholders, parses the JSON,
// simulates the file creation (without writing to disk), and then returns a tree view.
func GeneratePreviewFileTree(cmdName string, placeholders map[string]string, projectPath string) (string, error) {
	// Load command spec.
	spec := GetCommandSpec(cmdName)
	if spec.TemplatePath == "" {
		return "", fmt.Errorf("command %q has no template", cmdName)
	}

	// Load raw template bytes.
	data, err := LoadCommandTemplate(spec.TemplatePath)
	if err != nil {
		return "", fmt.Errorf("failed to load template: %w", err)
	}

	// Replace placeholders in the template.
	templateData := replacePlaceholders(string(data), placeholders)

	// Unmarshal into the JSONCommandTemplate structure.
	var tmpl JSONCommandTemplate
	if err := json.Unmarshal([]byte(templateData), &tmpl); err != nil {
		return "", fmt.Errorf("failed to parse template JSON: %w", err)
	}

	// Collect file paths that would be created. We aggregate file paths from each FilePathGroup.
	var filePaths []string
	for _, group := range tmpl.FilePaths {
		// Calculate the base path for the group.
		base := filepath.Join(projectPath, replacePlaceholders(group.Path, placeholders))
		// Recursively collect file paths from the tree nodes.
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
		// Check if the file at path is in the EditedIndexers map.
		if edited, ok := EditedIndexers[path]; ok && edited {
			return true
		}
		return false
	})
	return preview, nil
}

// -----------------------------------------------------------------------------
// New: GeneratePreviewFileTreeFromClipboard
// -----------------------------------------------------------------------------
// GeneratePreviewFileTreeFromClipboard reads the clipboard content (assumed to be a JSON
// template), applies the provided placeholders, and returns the preview file tree.
func GeneratePreviewFileTreeFromClipboard(placeholders map[string]string, projectPath string) (string, error) {
	clipboardContent, err := clipboard.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to read clipboard: %w", err)
	}

	templateData := replacePlaceholders(clipboardContent, placeholders)
	var tmpl JSONCommandTemplate
	if err := json.Unmarshal([]byte(templateData), &tmpl); err != nil {
		return "", fmt.Errorf("failed to parse clipboard JSON: %w", err)
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
		if edited, ok := EditedIndexers[path]; ok && edited {
			return true
		}
		return false
	})
	return preview, nil
}

// ExtractVariablesFromClipboard reads the clipboard content and extracts
// variable keys if it contains valid JSON template content.
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

	// If no variables found, return a default
	if len(result) == 0 {
		return []string{"Filename"}, nil
	}

	return result, nil
}
