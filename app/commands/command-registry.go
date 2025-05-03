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
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
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

// GetCommandVariableKeys checks command sources and returns required variable keys.
func GetCommandVariableKeys(cmdName, projectPath string, registry *project.ProjectRegistry) ([]string, error) {
	var jsonBytes []byte
	lowerCmdName := strings.ToLower(cmdName)

	if lowerCmdName == "paste from clipboard" {
		clipboardContent, readErr := clipboard.ReadAll()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read clipboard: %w", readErr)
		}
		jsonBytes = []byte(clipboardContent)
	} else if registry != nil && registry.ClipboardCommands != nil {
		if cmdSpec, found := registry.ClipboardCommands[cmdName]; found {
			jsonBytes = []byte(cmdSpec.Template)
		}
	}

	if jsonBytes == nil && projectPath != "" {
		localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands", cmdName+".json")
		if _, statErr := os.Stat(localCmdPath); statErr == nil {
			fileBytes, readErr := os.ReadFile(localCmdPath)
			if readErr != nil {
				return nil, fmt.Errorf("error reading project command file %s: %w", localCmdPath, readErr)
			}
			jsonBytes = fileBytes
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("error checking project command file %s: %w", localCmdPath, statErr)
		}
	}

	if jsonBytes == nil {
		spec := GetCommandSpec(cmdName)
		if spec.TemplatePath != "" {
			embeddedBytes, readErr := commandFiles.ReadFile(spec.TemplatePath)
			if readErr != nil {
				return nil, fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr)
			}
			jsonBytes = embeddedBytes
		}
	}

	if jsonBytes == nil {
		// If still no bytes, command doesn't exist or has no template
		// For commands without templates (like native ones), return empty list, no error.
		return []string{}, nil
	}

	// Now parse the found bytes
	keys, err := getTemplateVariableKeysFromBytes(jsonBytes)
	if err != nil {
		return nil, err // Propagate parsing error
	}

	return keys, nil
}

// RunCommand prepares a tea.Cmd to execute the command defined by a template.
// It looks for the command source in Clipboard -> Project Files -> Built-in.
// TODO: Add mechanism to detect required variables and prompt the user.
func RunCommand(cmdName, projectPath string, placeholders map[string]string, registry *project.ProjectRegistry) tea.Cmd {
	// Return a function that encapsulates the command execution logic.
	return func() tea.Msg {
		// Reset CreatedFiles and EditedIndexers for this run.
		CreatedFiles = []string{}
		EditedIndexers = make(map[string]bool)

		var jsonBytes []byte
		var executionSource string // To know if it came from clipboard or file

		// --- Determine Command Source and Load Template ---
		lowerCmdName := strings.ToLower(cmdName)

		if lowerCmdName == "paste from clipboard" {
			clipboardContent, readErr := clipboard.ReadAll()
			if readErr != nil {
				return app.ErrorMsg{Err: fmt.Errorf("failed to read clipboard: %w", readErr)}
			}
			templateData := replacePlaceholders(string(clipboardContent), placeholders)
			jsonBytes = []byte(templateData)
			executionSource = "clipboard"
		} else if registry != nil && registry.ClipboardCommands != nil {
			// 1. Check Clipboard Registry
			if cmdSpec, found := registry.ClipboardCommands[cmdName]; found {
				jsonBytes = []byte(cmdSpec.Template)
				executionSource = fmt.Sprintf("clipboard command '%s'", cmdName)
			}
		}

		// 2. Check Local Project Commands (if not found in clipboard)
		if jsonBytes == nil && projectPath != "" {
			localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands", cmdName+".json")
			if _, statErr := os.Stat(localCmdPath); statErr == nil {
				// File exists, try to read it
				fileBytes, readErr := os.ReadFile(localCmdPath)
				if readErr == nil {
					jsonBytes = fileBytes
					executionSource = fmt.Sprintf("project command file %s", cmdName+".json")
				} else {
					// File exists but couldn't read
					return app.ErrorMsg{Err: fmt.Errorf("error reading project command file %s: %w", localCmdPath, readErr)}
				}
			} else if !os.IsNotExist(statErr) {
				// Error checking file existence (other than not existing)
				return app.ErrorMsg{Err: fmt.Errorf("error checking project command file %s: %w", localCmdPath, statErr)}
			}
		}

		// 3. Check Built-in Commands (if not found elsewhere)
		if jsonBytes == nil {
			spec := GetCommandSpec(cmdName)
			if spec.TemplatePath != "" {
				embeddedBytes, readErr := commandFiles.ReadFile(spec.TemplatePath)
				if readErr == nil {
					jsonBytes = embeddedBytes
					executionSource = fmt.Sprintf("built-in template %s", spec.TemplatePath)
				} else {
					return app.ErrorMsg{Err: fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr)}
				}
			}
		}

		// Check if template was loaded
		if jsonBytes == nil {
			return app.ErrorMsg{Err: fmt.Errorf("command '%s' not found or template unavailable", cmdName)}
		}

		// --- Execute Template Logic ---
		execErr := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders)
		// Record history regardless of execution error

		// --- Record Command History (Common Logic) ---
		if registry != nil && projectPath != "" {
			if projectInfo, found := registry.GetProject(projectPath); found {
				// Use the original cmdName for history
				historicCmd := project.HistoricCommand{
					Name:           cmdName,
					Variables:      placeholders,
					Timestamp:      time.Now().Unix(),
					GeneratedFiles: append([]string{}, CreatedFiles...),
				}
				if projectInfo.CommandHistory == nil {
					projectInfo.CommandHistory = []project.HistoricCommand{}
				}
				projectInfo.CommandHistory = append(projectInfo.CommandHistory, historicCmd)
				// Limit history size
				if len(projectInfo.CommandHistory) > 20 {
					projectInfo.CommandHistory = projectInfo.CommandHistory[len(projectInfo.CommandHistory)-20:]
				}
				registry.AddOrUpdateProject(projectInfo)
				if saveErr := registry.Save(); saveErr != nil {
					fmt.Printf("Warning: Failed to save project registry after executing command '%s': %v\n", cmdName, saveErr)
				}
			} else {
				fmt.Printf("Warning: Project '%s' not found in registry, cannot record history.\n", projectPath)
			}
		} else {
			fmt.Println("Warning: Registry or ProjectPath unavailable, cannot record history.")
		}
		// --- End History Recording ---

		// Return result message
		if execErr != nil {
			return app.ErrorMsg{Err: fmt.Errorf("error executing template for command '%s' from %s: %w", cmdName, executionSource, execErr)}
		}

		// Success!
		return app.SuccessMsg{Message: fmt.Sprintf("Command '%s' executed successfully from %s.", cmdName, executionSource)}
	}
}
