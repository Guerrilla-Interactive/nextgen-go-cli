package commands

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
)

// Use *.json to embed JSON files in the same directory (non-recursively).
//
// If you want to embed subfolders, you can say go:embed **/*.json
//
//go:embed native-commands
var commandFiles embed.FS

// A registry to hold recognized JSON templates in memory
var templateRegistry = map[string][]byte{}

// Regex to find placeholders like {{.VarName}}, {{.PascalCaseVarName}}, etc.
// Captures the full identifier after the dot.
var placeholderRegex = regexp.MustCompile(`{{\s*\.\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*}}`)

func init() {
	// Walk the embedded FS and store each .json file in our registry map
	// Also collect discovered JSON paths for synthesizing folder-level commands
	discovered := []string{}
	err := fs.WalkDir(commandFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".json" {
			data, readErr := commandFiles.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("could not read file %s: %w", path, readErr)
			}
			// Store file contents in the registry, keyed by filename (like "page-and-archive.json")
			templateRegistry[path] = data
			discovered = append(discovered, path)
			// Populate Commands dynamically from embedded files
			type minimal struct {
				Title     string             `json:"title"`
				Name      string             `json:"name"`
				Slug      string             `json:"slug"`
				Show      *CommandVisibility `json:"show"`
				FilePaths []any              `json:"filePaths"`
				Run       []any              `json:"run"`
			}
			var m minimal
			_ = json.Unmarshal(data, &m) // ignore parse error for title extraction; fallback to filename
			name := strings.TrimSpace(m.Title)
			if name == "" {
				base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				name = strings.ReplaceAll(base, "-", " ")
			}
			lower := strings.ToLower(name)
			if !strings.HasPrefix(lower, "add ") && !strings.HasPrefix(lower, "remove ") {
				name = "add " + name
			}
			// Determine slug: prefer explicit JSON `slug`, else fall back to filename base
			slug := strings.TrimSpace(m.Slug)
			if slug == "" {
				slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}
			// Only register runnable commands (have filePaths or run)
			if len(m.FilePaths) > 0 || len(m.Run) > 0 {
				Commands = append(Commands, CommandSpec{Name: name, Slug: slug, TemplatePath: path, Visibility: m.Show})
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to init command registry: %v", err)
	}

	// Synthesize folder-level commands for native-commands/<category>/<bundle>
	dirsAdded := map[string]bool{}
	// Also synthesize category-level commands for native-commands/<category>
	categoriesAdded := map[string]bool{}
	for _, p := range discovered {
		// Expect paths like native-commands/<category>/<bundle>/.../file.json
		parts := strings.Split(p, "/")
		if len(parts) < 4 {
			continue
		}
		if parts[0] != "native-commands" {
			continue
		}

		// --- Category-level synthetic command: native-commands/<category>
		// Category folders are identifiers only; do not register as commands
		categoryKey := strings.Join(parts[:2], "/") // native-commands/<category>
		category := parts[1]
		if !categoriesAdded[categoryKey] {
			categoriesAdded[categoryKey] = true
		}

		folderKey := strings.Join(parts[:3], "/") // native-commands/nextjs/add-index-and-slug
		if dirsAdded[folderKey] {
			continue
		}
		dirsAdded[folderKey] = true
		// Build synthetic template bytes with autoBrowseRoot (for future browsing) or minimal marker
		bundle := parts[2]
		category = parts[1]
		// Title derived from bundle
		title := strings.ReplaceAll(bundle, "-", " ")
		if !strings.HasPrefix(strings.ToLower(title), "add ") && !strings.HasPrefix(strings.ToLower(title), "remove ") {
			title = "add " + title
		}
		slug := category + "-" + bundle
		tmpl := fmt.Sprintf(`{"_type":"command","title":"%s","slug":"%s","autoBrowseRoot":"%s"}`, strings.Title(title), slug, folderKey)
		key := "auto/" + slug + ".json"
		templateRegistry[key] = []byte(tmpl)

		// Restrict visibility of bundle wrappers to projects that declare the category
		// either in .nextgen/command-packages.json or package.json nextgen-identifiers
		Commands = append(Commands, CommandSpec{
			Name:         title,
			Slug:         slug,
			TemplatePath: key,
			Visibility: &CommandVisibility{
				AnyOf: []CommandVisibilityClause{
					{CommandPackagesContains: []string{category}},
					{PackageJSONArrayContains: map[string]string{"nextgen-identifiers": category}},
				},
			},
		})
	}

	// Ensure stable ordering for UI lists
	sort.Slice(Commands, func(i, j int) bool { return Commands[i].Name < Commands[j].Name })
}

// FSChild represents a child entry in the embedded native-commands tree.
type FSChild struct {
	Name  string
	Path  string
	IsDir bool
}

// ListNativeChildren lists directories and .json files under the given embedded prefix path.
// Example prefix: "native-commands/nextjs/add-index-and-slug".
func ListNativeChildren(prefix string) ([]FSChild, error) {
	entries, err := fs.ReadDir(commandFiles, prefix)
	if err != nil {
		return nil, err
	}
	var out []FSChild
	for _, e := range entries {
		p := filepath.ToSlash(filepath.Join(prefix, e.Name()))
		if e.IsDir() {
			out = append(out, FSChild{Name: e.Name(), Path: p, IsDir: true})
			continue
		}
		if filepath.Ext(e.Name()) == ".json" {
			out = append(out, FSChild{Name: e.Name(), Path: p, IsDir: false})
		}
	}
	return out, nil
}

// ReadEmbeddedTemplate returns the raw bytes of an embedded template path.
func ReadEmbeddedTemplate(path string) ([]byte, error) {
	return commandFiles.ReadFile(path)
}

// FindFirstJSONUnder returns the path to the first JSON file found under prefix (depth-first).
func FindFirstJSONUnder(prefix string) (string, bool) {
	entries, err := fs.ReadDir(commandFiles, prefix)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			if p, ok := FindFirstJSONUnder(filepath.ToSlash(filepath.Join(prefix, e.Name()))); ok {
				return p, true
			}
		} else if filepath.Ext(e.Name()) == ".json" {
			return filepath.ToSlash(filepath.Join(prefix, e.Name())), true
		}
	}
	return "", false
}

// FindCommandByTemplatePath returns the registered CommandSpec for an embedded template path.
func FindCommandByTemplatePath(path string) (CommandSpec, bool) {
	for _, c := range Commands {
		if c.TemplatePath == path {
			return c, true
		}
	}
	return CommandSpec{}, false
}

// LoadCommandTemplate retrieves the raw JSON template data from memory.
// The path should match what's stored in templateRegistry (e.g. "app/commands/page-and-archive.json").
func LoadCommandTemplate(path string) ([]byte, error) {
	data, found := templateRegistry[path]
	if !found {
		return nil, fmt.Errorf("template %q not found in registry", path)
	}
	return data, nil
}

// GetCommandSpec returns the CommandSpec for a given command name.
func GetCommandSpec(cmdName string) CommandSpec {
	for _, spec := range Commands {
		if spec.Name == cmdName {
			return spec
		}
	}
	// Try slug match using exact or kebab-cased variant
	normalized := ToKebabCase(cmdName)
	for _, spec := range Commands {
		if strings.EqualFold(spec.Slug, cmdName) || strings.EqualFold(spec.Slug, normalized) {
			return spec
		}
	}
	return CommandSpec{}
}

// CommandSpec defines a single command's name, an optional JSON template path,
// and (optionally) a list of variable keys. If VariableKeys is non-empty then
// multiple independent variables will be collected.
type CommandSpec struct {
	Name         string
	Slug         string
	TemplatePath string
	Visibility   *CommandVisibility
}

// Commands is our single authoritative list of all possible commands.
var Commands = []CommandSpec{}

// RecentUsed & NextSteps remain separate slices, for usage in the UI.
var RecentUsed = []string{}

var NextSteps = []string{
	"Show all my commands",
	"LogoutOrLoginPlaceholder",
}

// CommandWithIcon returns a user-friendly label with an icon prefix.
// It automatically assigns a plus sign (✚) for commands starting with "add "
// and an X (✖) for commands starting with "remove ".
func CommandWithIcon(cmd string) string {
	lowerCmd := strings.ToLower(cmd)
	if strings.HasPrefix(lowerCmd, "add ") {
		return fmt.Sprintf("✚  %s", cmd)
	}
	if strings.HasPrefix(lowerCmd, "remove ") {
		return fmt.Sprintf("✖  %s", cmd)
	}
	return fmt.Sprintf("•  %s", cmd)
}

// AllCommandNames returns the command names in the order they appear in Commands.
func AllCommandNames() []string {
	names := make([]string, len(Commands))
	for i, c := range Commands {
		names[i] = c.Name
	}
	return names
}

// TemplatePathFor looks up the first command in Commands with the given name
// and returns its TemplatePath (plus true if found).
func TemplatePathFor(cmdName string) (string, bool) {
	for _, c := range Commands {
		if c.Name == cmdName {
			return c.TemplatePath, true
		}
	}
	// Try by slug
	normalized := ToKebabCase(cmdName)
	for _, c := range Commands {
		if strings.EqualFold(c.Slug, cmdName) || strings.EqualFold(c.Slug, normalized) {
			return c.TemplatePath, true
		}
	}
	return "", false
}

// InferVariableKeys scans content for placeholders like {{.VarName}}
// and returns a unique, sorted list of the base variable names found.
func InferVariableKeys(content string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(content, -1)
	keys := make(map[string]bool)

	// Known prefixes to strip
	prefixes := []string{"PascalCase", "CamelCase", "KebabCase", "LowerCase", "UpperCase"}

	for _, match := range matches {
		if len(match) > 1 {
			fullIdentifier := match[1]
			baseName := fullIdentifier
			// Attempt to strip known prefixes
			for _, prefix := range prefixes {
				if strings.HasPrefix(baseName, prefix) {
					// Ensure the part after the prefix starts with an uppercase letter
					// or that the prefix matches the whole identifier (e.g. {{.Name}} vs {{.PascalCaseName}})
					suffix := baseName[len(prefix):]
					if len(suffix) > 0 && (suffix[0] >= 'A' && suffix[0] <= 'Z') {
						baseName = suffix
						break // Found prefix, stop checking
					} else if len(suffix) == 0 { // Handle cases like {{.PascalCase}} where prefix IS the name
						// Keep baseName as the prefix itself in this case
						break
					}
				}
			}
			keys[baseName] = true // Add the derived base name
		}
	}

	var uniqueKeys []string
	for k := range keys {
		uniqueKeys = append(uniqueKeys, k)
	}
	return uniqueKeys
}

// getTemplateVariableKeysFromBytes parses template bytes and infers variable keys.
func getTemplateVariableKeysFromBytes(templateBytes []byte) ([]string, error) {
	var genericData interface{}
	if err := json.Unmarshal(templateBytes, &genericData); err != nil {
		return nil, fmt.Errorf("failed to parse generic template JSON: %w", err)
	}

	allKeys := make(map[string]bool)

	// Recursive function to traverse the parsed JSON data
	var traverse func(data interface{})
	traverse = func(data interface{}) {
		switch value := data.(type) {
		case map[string]interface{}:
			// If it's a map, iterate through its key-value pairs
			for key, v := range value {
				// If the key is "name" or "code" and the value is a string, infer keys
				if key == "name" || key == "code" {
					if strVal, ok := v.(string); ok {
						for _, inferredKey := range InferVariableKeys(strVal) {
							allKeys[inferredKey] = true
						}
					}
				}
				// Recursively traverse the value
				traverse(v)
			}
		case []interface{}:
			// If it's a slice, iterate through its elements and traverse recursively
			for _, item := range value {
				traverse(item)
			}
			// Ignore other types (string, number, bool, nil)
		}
	}

	// Start traversal from the root of the parsed data
	traverse(genericData)

	// Convert map keys to slice
	finalKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		finalKeys = append(finalKeys, k)
	}
	return finalKeys, nil
}

// GetCommandVariableKeys attempts to determine the required variable keys for a command.
// It checks clipboard, built-in templates, and local project commands.
func GetCommandVariableKeys(cmdName, projectPath string, registry *project.ProjectRegistry) ([]string, error) {
	// 1. Handle clipboard command
	if strings.ToLower(cmdName) == "paste from clipboard" {
		return ExtractVariablesFromClipboard() // Uses helper from command-helpers.go
	}

	// 2. Check built-in commands
	spec := GetCommandSpec(cmdName)
	if spec.TemplatePath != "" {
		templateBytes, err := LoadCommandTemplate(spec.TemplatePath)
		if err != nil {
			return nil, fmt.Errorf("error loading built-in template %s: %w", spec.TemplatePath, err)
		}
		return getTemplateVariableKeysFromBytes(templateBytes)
	}
	// 2b. If cmdName looks like an embedded template path, try loading directly (auto-browse file)
	if strings.HasSuffix(strings.ToLower(cmdName), ".json") {
		if templateBytes, err := LoadCommandTemplate(cmdName); err == nil {
			return getTemplateVariableKeysFromBytes(templateBytes)
		}
	}

	// 3. Check project-local commands
	if projectPath != "" && projectPath != "." {
		localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
		kebabName := ToKebabCase(cmdName)
		cmdFilePath := filepath.Join(localCmdDir, kebabName+".json")
		if _, err := os.Stat(cmdFilePath); err == nil {
			// File exists, read and parse
			projectCmdBytes, readErr := os.ReadFile(cmdFilePath)
			if readErr != nil {
				return nil, fmt.Errorf("error reading project command file %s: %w", cmdFilePath, readErr)
			}
			return getTemplateVariableKeysFromBytes(projectCmdBytes)
		}
	}

	// 4. Check user-saved clipboard commands (if registry available)
	if registry != nil && registry.ClipboardCommands != nil {
		if clipSpec, found := registry.ClipboardCommands[cmdName]; found {
			return getTemplateVariableKeysFromBytes([]byte(clipSpec.Template))
		}
	}

	// 5. Command not found or doesn't use templates that require variables
	return nil, nil // Return nil, nil if no keys applicable or command not found
}
