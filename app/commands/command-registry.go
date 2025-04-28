package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
)

// Use *.json to embed JSON files in the same directory (non-recursively).
//
// If you want to embed subfolders, you can say go:embed **/*.json
//
//go:embed *.json
var commandFiles embed.FS

// A registry to hold recognized JSON templates in memory
var templateRegistry = map[string][]byte{}

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
	{Name: "add section"}, // no template (placeholder)
	{Name: "remove section"},
	{Name: "add page", TemplatePath: "page-and-archive.json"},
	{Name: "add wordpress block", TemplatePath: "wordpress-interactive-block-for-nextgen-theme.json"},
	{Name: "add nextgen pagebuilder block", TemplatePath: "add-nextgen-pagebuilder-block.json"},
	{Name: "add multiple variables example", TemplatePath: "multiple-variables-example.json"},
	{Name: "add wordpress gutenberg block", TemplatePath: "wordpress-gutenberg-block.json"},
	{Name: "add test pagebuilder block", TemplatePath: "test-pagebuilder.json"},
	{Name: "add nextgen slug route", TemplatePath: "add-nextgen-slug-route.json"},
	{Name: "remove page"},
	{Name: "add portable-component"},
	{Name: "remove portable-component"},
	{Name: "add component"},
	{Name: "remove component"},
	{Name: "add schema"},
	{Name: "remove schema"},
	{Name: "add query"},
	{Name: "remove query"},
	{Name: "add sanity-plugin"},
	{Name: "remove sanity-plugin"},
	{Name: "undo"},
	{Name: "redo"},
	{Name: "add hello", TemplatePath: "app/commands/hello-world.json"},
	{Name: "paste from clipboard"},
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

// RunCommand executes the command defined by the JSON template.
// It now accepts the registry to record history after execution.
func RunCommand(cmdName, projectPath string, placeholders map[string]string, registry *project.ProjectRegistry) error {
	// Reset CreatedFiles and EditedIndexers for this run.
	CreatedFiles = []string{}
	EditedIndexers = make(map[string]bool)

	spec := GetCommandSpec(cmdName)
	if spec.TemplatePath == "" {
		return fmt.Errorf("command '%s' not found or has no template path", cmdName)
	}

	// Read the template content.
	jsonBytes, err := commandFiles.ReadFile(spec.TemplatePath)
	if err != nil {
		return fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, err)
	}

	// Execute the template logic (creates/modifies files).
	err = ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders)
	if err != nil {
		// Return the execution error, but still try to record history if needed?
		// Or maybe only record history on success? Let's record regardless for now.
		// return fmt.Errorf("error executing template for command '%s': %w", cmdName, err)
	}

	// --- Record Command History (Moved Here) ---
	// Log the placeholders received by RunCommand *before* recording history
	fmt.Printf("DEBUG: RunCommand received placeholders: %+v\n", placeholders)

	// Record even if ExecuteJSONTemplateFromMemory returned an error,
	// as the user initiated the command.
	if registry != nil && projectPath != "" {
		if projectInfo, found := registry.GetProject(projectPath); found {
			historicCmd := project.HistoricCommand{
				Name:           cmdName,
				Variables:      placeholders, // Store the actual variables used!
				Timestamp:      time.Now().Unix(),
				GeneratedFiles: append([]string{}, CreatedFiles...), // Copy slice
			}
			if projectInfo.CommandHistory == nil {
				projectInfo.CommandHistory = []project.HistoricCommand{}
			}
			projectInfo.CommandHistory = append(projectInfo.CommandHistory, historicCmd)
			// Limit history size
			if len(projectInfo.CommandHistory) > 20 { // Use a constant later?
				projectInfo.CommandHistory = projectInfo.CommandHistory[len(projectInfo.CommandHistory)-20:]
			}
			registry.AddOrUpdateProject(projectInfo) // Update registry (also updates usage count)
			if saveErr := registry.Save(); saveErr != nil {
				// Log non-fatal error
				fmt.Printf("Warning: Failed to save project registry after executing command '%s': %v\n", cmdName, saveErr)
			}
		} else {
			fmt.Printf("Warning: Project '%s' not found in registry, cannot record history.\n", projectPath)
		}
	} else {
		fmt.Println("Warning: Registry or ProjectPath unavailable, cannot record history.")
	}
	// --- End History Recording ---

	// Return the original execution error, if any
	if err != nil {
		return fmt.Errorf("error executing template for command '%s': %w", cmdName, err)
	}

	return nil
}
