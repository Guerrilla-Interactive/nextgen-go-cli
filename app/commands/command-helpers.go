package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	// Needed for timestamp in history
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project" // Needed for registry
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
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
	FilePaths      []FilePathGroup `json:"filePaths"`
	Args           []ArgDef        `json:"args"`
	Run            []RunStep       `json:"run"`
	AutoBrowseRoot string          `json:"autoBrowseRoot"`
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

// ArgDef describes a variable to ask the user for.
type ArgDef struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // text, select
	Message      string   `json:"message"`
	Choices      []Choice `json:"choices"`
	Default      string   `json:"default"`
	Required     bool     `json:"required"`
	RequiredWhen *struct {
		Var    string `json:"var"`
		Equals string `json:"equals"`
	} `json:"requiredWhen"`
}

type Choice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RunStep allows invoking a subcommand based on a condition.
type RunStep struct {
	Type        string   `json:"type"` // invoke
	Slug        string   `json:"slug"`
	When        string   `json:"when"`
	ForwardVars []string `json:"forwardVars"`
}

// -----------------------------------------------------------------------------
// Command visibility conditions
// -----------------------------------------------------------------------------

// CommandVisibility defines optional conditions for when a command should be shown.
// Currently supports simple top-level package.json key equality checks.
type CommandVisibility struct {
	// PackageJSON matches top-level properties in package.json by exact string equality.
	// Example: { "name": "nextjs" }
	PackageJSON map[string]string `json:"packageJson"`
	// PackageJSONArrayContains asserts that a top-level array contains a string.
	// Example: { "nextgen-identifiers": "nextjs" }
	PackageJSONArrayContains map[string]string `json:"packageJsonArrayContains"`
	// AnyOf allows OR-combined clauses.
	AnyOf []CommandVisibilityClause `json:"anyOf"`
	// NextGen command packages file contains all of these identifiers (.nextgen/command-packages.json)
	CommandPackagesContains []string `json:"commandPackagesContains"`
}

// CommandVisibilityClause represents a single OR clause.
type CommandVisibilityClause struct {
	PackageJSON              map[string]string `json:"packageJson"`
	PackageJSONArrayContains map[string]string `json:"packageJsonArrayContains"`
	CommandPackagesContains  []string          `json:"commandPackagesContains"`
}

// isPackageJSONMatch returns true if all expected top-level keys in package.json equal the provided values.
func isPackageJSONMatch(projectPath string, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	pkgPath := filepath.Join(projectPath, "package.json")
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return false
	}
	for k, v := range expected {
		if actual, ok := data[k]; ok {
			switch t := actual.(type) {
			case string:
				if strings.TrimSpace(t) != strings.TrimSpace(v) {
					return false
				}
			default:
				if fmt.Sprint(t) != v {
					return false
				}
			}
		} else {
			return false
		}
	}
	return true
}

// isPackageJSONArrayContains returns true if each key names an array containing the wanted string.
func isPackageJSONArrayContains(projectPath string, expected map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	pkgPath := filepath.Join(projectPath, "package.json")
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}
	var data map[string]any
	if err := json.Unmarshal(b, &data); err != nil {
		return false
	}
	for key, want := range expected {
		raw, ok := data[key]
		if !ok {
			return false
		}
		arr, ok := raw.([]any)
		if !ok {
			return false
		}
		found := false
		for _, v := range arr {
			if s, ok := v.(string); ok && strings.TrimSpace(s) == strings.TrimSpace(want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// isCommandPackagesContains returns true if .nextgen/command-packages.json contains all expected tokens.
// Supported formats:
// - ["nextjs", "react"]
// - { "identifiers": ["nextjs", ...] }
func isCommandPackagesContains(projectPath string, expected []string) bool {
	if len(expected) == 0 {
		return true
	}
	p := filepath.Join(projectPath, ".nextgen", "command-packages.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	trim := strings.TrimSpace(string(b))
	if trim == "" {
		return false
	}
	// Try array of strings first
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
		set := make(map[string]bool, len(arr))
		for _, s := range arr {
			set[strings.TrimSpace(s)] = true
		}
		for _, want := range expected {
			if !set[strings.TrimSpace(want)] {
				return false
			}
		}
		return true
	}
	// Try object with identifiers array
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err == nil {
		collected := map[string]bool{}
		if raw, ok := obj["identifiers"]; ok {
			if a, ok2 := raw.([]any); ok2 {
				for _, v := range a {
					if s, ok3 := v.(string); ok3 {
						collected[strings.TrimSpace(s)] = true
					}
				}
			}
		}
		// Fallback: collect any string arrays in the object
		if len(collected) == 0 {
			for _, v := range obj {
				if a, ok2 := v.([]any); ok2 {
					for _, vv := range a {
						if s, ok3 := vv.(string); ok3 {
							collected[strings.TrimSpace(s)] = true
						}
					}
				}
			}
		}
		if len(collected) > 0 {
			for _, want := range expected {
				if !collected[strings.TrimSpace(want)] {
					return false
				}
			}
			return true
		}
	}
	return false
}

func matchesVisibilityClause(projectPath string, clause CommandVisibilityClause) bool {
	if len(clause.PackageJSON) > 0 && !isPackageJSONMatch(projectPath, clause.PackageJSON) {
		return false
	}
	if len(clause.PackageJSONArrayContains) > 0 && !isPackageJSONArrayContains(projectPath, clause.PackageJSONArrayContains) {
		return false
	}
	if len(clause.CommandPackagesContains) > 0 && !isCommandPackagesContains(projectPath, clause.CommandPackagesContains) {
		return false
	}
	return true
}

// IsCommandVisible evaluates whether a command should be shown for the given project path.
func IsCommandVisible(spec CommandSpec, projectPath string) bool {
	// No conditions implies visible
	if spec.Visibility == nil {
		return true
	}
	// AnyOf clauses (OR)
	if len(spec.Visibility.AnyOf) > 0 {
		for _, c := range spec.Visibility.AnyOf {
			if matchesVisibilityClause(projectPath, c) {
				return true
			}
		}
		return false
	}
	// AND of top-level fields
	if len(spec.Visibility.PackageJSON) > 0 {
		if !isPackageJSONMatch(projectPath, spec.Visibility.PackageJSON) {
			return false
		}
	}
	if len(spec.Visibility.PackageJSONArrayContains) > 0 {
		if !isPackageJSONArrayContains(projectPath, spec.Visibility.PackageJSONArrayContains) {
			return false
		}
	}
	if len(spec.Visibility.CommandPackagesContains) > 0 {
		if !isCommandPackagesContains(projectPath, spec.Visibility.CommandPackagesContains) {
			return false
		}
	}
	return true
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
				if cli.IsVerboseEnabled() {
					fmt.Printf("ℹ️  Detected indexer marker in file %s, registering as an indexer file.\n", currentPath)
				}
			}

			// If file already exists then we introduce smart merge behavior.
			if _, err := os.Stat(currentPath); err == nil {
				if isIndexer {
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
					if cli.IsVerboseEnabled() {
						fmt.Printf("✓ Merged updates into existing file %s.\n", currentPath)
					}
					if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
						MarkEditedIndexer(rel)
					} else {
						MarkEditedIndexer(currentPath)
					}
				} else {
					newContent := removeSnippetMarkers(code)
					if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil {
						return fmt.Errorf("failed to overwrite file %s: %w", currentPath, err)
					}
					if cli.IsVerboseEnabled() {
						fmt.Printf("✓ Replaced existing file %s.\n", currentPath)
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

// RunJsonTemplate loads and executes a command template from a JSON file.
func RunJsonTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplate(jsonFilePath, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template: %w", err)
	}
	return nil
}

// RunJsonTemplateBytes loads and executes a command template from byte data.
func RunJsonTemplateBytes(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template from memory: %w", err)
	}
	return nil
}

// ExecuteJSONTemplateFromMemory executes the template logic given the JSON bytes.
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

// ToKebabCase converts a string to kebab-case.
func ToKebabCase(input string) string {
	words := splitIntoWords(input)
	return strings.ToLower(strings.Join(words, "-"))
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
// File Tree Preview Generation
// ----------------------------------------------------------------------------

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

	// If no variables found, return empty to signal no prompt needed
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

// -----------------------------------------------------------------------------
// Argument Validation Helper
// -----------------------------------------------------------------------------

// ValidateArgs checks the provided CommandArgs against the command's definitions.
func ValidateArgs(parsedArgs cli.CommandArgs, expectedArgs []cli.ArgDef, expectedFlags []cli.FlagDef) error {
	// 1. Check required positional arguments
	requiredArgCount := 0
	for _, argDef := range expectedArgs {
		if argDef.Required {
			requiredArgCount++
		}
	}
	allowsTrailingArgs := false
	if len(expectedArgs) > 0 && strings.HasSuffix(expectedArgs[len(expectedArgs)-1].Name, "...") {
		allowsTrailingArgs = true
	}

	if len(parsedArgs.Variables) < requiredArgCount {
		// Construct a meaningful error message based on expected args
		var requiredNames []string
		for i := 0; i < requiredArgCount; i++ {
			requiredNames = append(requiredNames, fmt.Sprintf("<%s>", expectedArgs[i].Name))
		}
		return fmt.Errorf("missing required arguments: %s", strings.Join(requiredNames, " "))
	}

	// Check if too many args were provided, unless trailing args are allowed
	if !allowsTrailingArgs && len(parsedArgs.Variables) > len(expectedArgs) {
		return fmt.Errorf("too many arguments provided. Expected max %d, got %d", len(expectedArgs), len(parsedArgs.Variables))
	}

	// 2. Check required flags
	for _, flagDef := range expectedFlags {
		if flagDef.Required {
			_, longExists := parsedArgs.Flags[flagDef.Name]
			_, shortExists := parsedArgs.Flags[flagDef.ShortName]
			_, longBoolExists := parsedArgs.BoolFlags[flagDef.Name]
			_, shortBoolExists := parsedArgs.BoolFlags[flagDef.ShortName]

			found := longExists || (flagDef.ShortName != "" && shortExists) || longBoolExists || (flagDef.ShortName != "" && shortBoolExists)

			if !found {
				flagName := "--" + flagDef.Name
				if flagDef.ShortName != "" {
					flagName += "/-" + flagDef.ShortName
				}
				return fmt.Errorf("missing required flag: %s", flagName)
			}
		}
	}

	return nil // Validation passed
}

// RunCommand handles the asynchronous execution of commands triggered from the TUI.
// It determines the command type (clipboard, project, built-in) and executes it.
// It returns a tea.Cmd that will send an app.CommandFinishedMsg when done.
func RunCommand(cmdName, projectPath string, placeholders map[string]string, registry *project.ProjectRegistry) tea.Cmd {
	// Reset global file/edit trackers before running the command
	CreatedFiles = []string{}
	EditedIndexers = make(map[string]bool)

	// Make a copy of placeholders to avoid modification issues
	localPlaceholders := make(map[string]string)
	if placeholders != nil {
		for k, v := range placeholders {
			localPlaceholders[k] = v
		}
	}

	// Return the async command function
	return func() tea.Msg {
		var err error
		var executionSource string // For potential error messages
		var templateBytes []byte

		// --- Special Handling for Paste From Clipboard ---
		if strings.ToLower(cmdName) == "paste from clipboard" {
			clipboardContent, readErr := clipboard.ReadAll()
			if readErr != nil {
				err = fmt.Errorf("failed to read clipboard for paste command: %w", readErr)
			} else {
				templateBytes = []byte(clipboardContent)
				executionSource = "clipboard content"
			}
		} else if strings.HasSuffix(strings.ToLower(cmdName), ".json") {
			// Execute embedded template by its full path
			if embeddedBytes, readErr := LoadCommandTemplate(cmdName); readErr == nil {
				templateBytes = embeddedBytes
				executionSource = "embedded path"
			} else {
				err = fmt.Errorf("template path %s not found: %w", cmdName, readErr)
			}
		} else {
			// --- Original Logic to Find Template ---
			// 1. Check Clipboard Registry Commands (if registry is available)
			if registry != nil && registry.ClipboardCommands != nil {
				if clipSpec, found := registry.ClipboardCommands[cmdName]; found {
					templateBytes = []byte(clipSpec.Template)
					executionSource = fmt.Sprintf("clipboard command '%s'", cmdName)
				}
			}

			// 2. Check Project-Local Commands (if not found above and path is valid)
			if templateBytes == nil && projectPath != "" && projectPath != "." {
				localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands")
				kebabName := ToKebabCase(cmdName)
				cmdFilePath := filepath.Join(localCmdPath, kebabName+".json")
				if _, statErr := os.Stat(cmdFilePath); statErr == nil {
					fileBytes, readErr := os.ReadFile(cmdFilePath)
					if readErr == nil {
						templateBytes = fileBytes
						executionSource = fmt.Sprintf("project command '%s'", kebabName+".json")
					} else {
						err = fmt.Errorf("error reading project command file %s: %w", cmdFilePath, readErr)
					}
				} else if !os.IsNotExist(statErr) {
					err = fmt.Errorf("error checking project command file %s: %w", cmdFilePath, statErr)
				}
			}

			// 3. Check Built-in Commands (if not found above)
			if templateBytes == nil && err == nil { // Only check if no bytes found and no prior error
				spec := GetCommandSpec(cmdName)
				if spec.TemplatePath != "" {
					embeddedBytes, readErr := LoadCommandTemplate(spec.TemplatePath) // Use LoadCommandTemplate
					if readErr == nil {
						templateBytes = embeddedBytes
						executionSource = fmt.Sprintf("built-in template %s", spec.TemplatePath)
					} else {
						err = fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr)
					}
				}
			}
		}

		// --- Execute if template found and no error so far ---
		if templateBytes != nil && err == nil {
			err = ExecuteJSONTemplateFromMemory(templateBytes, projectPath, localPlaceholders)
			if err != nil {
				// Add context to the execution error
				err = fmt.Errorf("error executing template for command '%s' from %s: %w", cmdName, executionSource, err)
			}
		} else if err == nil {
			// No template found, and no other error occurred
			err = fmt.Errorf("command '%s' not found or has no associated template for TUI execution", cmdName)
		}

		// --- Return CommandFinishedMsg ---
		// Always return the message, populated with execution details.
		// History recording happens in main.go based on this message.
		return app.CommandFinishedMsg{
			Err:            err, // Will be nil on success
			CommandName:    cmdName,
			ProjectPath:    projectPath,
			Placeholders:   localPlaceholders,
			GeneratedFiles: append([]string{}, CreatedFiles...), // Send a copy
		}
	}
}

// UpsertClipboardCommand overwrites or adds a clipboard command by name and saves the registry.
func UpsertClipboardCommand(registry *project.ProjectRegistry, name string, template string) error {
	if registry == nil {
		return fmt.Errorf("registry unavailable")
	}
	if registry.ClipboardCommands == nil {
		registry.ClipboardCommands = make(map[string]project.ClipboardCommandSpec)
	}
	registry.ClipboardCommands[name] = project.ClipboardCommandSpec{
		Name:       name,
		Template:   template,
		IsFavorite: registry.ClipboardCommands[name].IsFavorite, // preserve favorite if existed
		Timestamp:  time.Now().Unix(),
	}
	return registry.Save()
}

func LoadTemplateBytesForName(cmdName, projectPath string, registry *project.ProjectRegistry) ([]byte, string, error) {
	// 1. Clipboard registry
	if registry != nil && registry.ClipboardCommands != nil {
		if clipSpec, found := registry.ClipboardCommands[cmdName]; found {
			return []byte(clipSpec.Template), "clipboard", nil
		}
	}
	// 2. Project-local
	if projectPath != "" && projectPath != "." {
		localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands")
		kebabName := ToKebabCase(cmdName)
		cmdFilePath := filepath.Join(localCmdPath, kebabName+".json")
		if _, statErr := os.Stat(cmdFilePath); statErr == nil {
			fileBytes, readErr := os.ReadFile(cmdFilePath)
			if readErr == nil {
				return fileBytes, "project", nil
			}
			return nil, "", fmt.Errorf("error reading project command file %s: %w", cmdFilePath, readErr)
		}
	}
	// 3. Built-in by name/slug
	spec := GetCommandSpec(cmdName)
	if spec.TemplatePath != "" {
		embeddedBytes, readErr := LoadCommandTemplate(spec.TemplatePath)
		if readErr == nil {
			return embeddedBytes, "builtin", nil
		}
		return nil, "", fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr)
	}
	return nil, "", fmt.Errorf("template not found for %s", cmdName)
}

// IsCompositeTemplate returns true if the template JSON defines run steps without filePaths.
func IsCompositeTemplate(templateBytes []byte) bool {
	var t struct {
		FilePaths []any     `json:"filePaths"`
		Run       []RunStep `json:"run"`
	}
	if err := json.Unmarshal(templateBytes, &t); err != nil {
		return false
	}
	return len(t.FilePaths) == 0 && len(t.Run) > 0
}

// GetCompositeRunSlugs returns the list of slugs referenced by run steps.
func GetCompositeRunSlugs(templateBytes []byte) ([]string, error) {
	var t struct {
		Run []RunStep `json:"run"`
	}
	if err := json.Unmarshal(templateBytes, &t); err != nil {
		return nil, err
	}
	var slugs []string
	for _, s := range t.Run {
		if strings.ToLower(s.Type) == "invoke" && strings.TrimSpace(s.Slug) != "" {
			slugs = append(slugs, s.Slug)
		}
	}
	return slugs, nil
}

// ResolveCommandTitleBySlug returns a friendly name for a command identified by slug or name.
func ResolveCommandTitleBySlug(nameOrSlug string) string {
	spec := GetCommandSpec(nameOrSlug)
	if spec.Name != "" {
		return spec.Name
	}
	// Fallback to capitalized slug
	parts := strings.Split(ToKebabCase(nameOrSlug), "-")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, " ")
}
