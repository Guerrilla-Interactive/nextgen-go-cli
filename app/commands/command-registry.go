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
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
)

// Use *.json to embed JSON files in the same directory (non-recursively).
//
// If you want to embed subfolders, you can say go:embed **/*.json
//
//go:embed native-commands/*.json
var commandFiles embed.FS

// A registry to hold recognized JSON templates in memory
var templateRegistry = map[string][]byte{}

// Regex to find placeholders like {{.VarName}}, {{.PascalCaseVarName}}, etc.
// Captures the full identifier after the dot.
var placeholderRegex = regexp.MustCompile(`{{\s*\.\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*}}`)

func init() {
	// Walk the embedded FS and store each .json file in our registry map
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
			// log.Printf("Embedded file: %s", path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to init command registry: %v", err)
	}
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
	return CommandSpec{}
}

// CommandSpec defines a single command's name, an optional JSON template path,
// and (optionally) a list of variable keys. If VariableKeys is non-empty then
// multiple independent variables will be collected.
type CommandSpec struct {
	Name         string
	TemplatePath string
}

// Commands is our single authoritative list of all possible commands.
var Commands = []CommandSpec{
	{Name: "add page", TemplatePath: "native-commands/page-and-archive.json"},
	{Name: "add wordpress block", TemplatePath: "native-commands/wordpress-interactive-block-for-nextgen-theme.json"},
	{Name: "add nextgen pagebuilder block", TemplatePath: "native-commands/add-nextgen-pagebuilder-block.json"},
	{Name: "add multiple variables example", TemplatePath: "native-commands/multiple-variables-example.json"},
	{Name: "add wordpress gutenberg block", TemplatePath: "native-commands/wordpress-gutenberg-block.json"},
	{Name: "add test pagebuilder block", TemplatePath: "native-commands/test-pagebuilder.json"},
	{Name: "add nextgen slug route", TemplatePath: "native-commands/add-nextgen-slug-route.json"},
	{Name: "undo"},
	{Name: "redo"},
	{Name: "add hello", TemplatePath: "native-commands/hello-world.json"},
}

// RecentUsed & NextSteps remain separate slices, for usage in the UI.
var RecentUsed = []string{
	"paste from clipboard",
	"add wordpress block",
	"add wordpress gutenberg block",
	"add nextgen pagebuilder block",
	"add nextgen slug route",
	"add multiple variables example",
	"add test pagebuilder block",

	"add page",
	"undo",
	"redo",
}

var NextSteps = []string{
	"Show all my commands",
	"LogoutOrLoginPlaceholder",
}

// CommandIconMap associates non-add/remove commands with an icon.
// The "add" and "remove" commands are now handled automatically.
var CommandIconMap = map[string]string{
	"undo":                 "↺",
	"redo":                 "↻",
	"paste from clipboard": "📋",
	"view project stats":   "📦",
	// Other commands that do not start with "add " or "remove " can be added here.
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
	if icon, ok := CommandIconMap[cmd]; ok {
		return fmt.Sprintf("%s  %s", icon, cmd)
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

// ExecuteCommandRegistryCommand handles the execution of template-based commands asynchronously.
// Renamed from RunCommand to avoid conflict.
// REMOVED as RunCommand in command-helpers.go now handles TUI execution.
/*
func ExecuteCommandRegistryCommand(cmdName, projectPath string, placeholders map[string]string, registry *project.ProjectRegistry) tea.Cmd {
	// ... function body removed ...
}
*/
