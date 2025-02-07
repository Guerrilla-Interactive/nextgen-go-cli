package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
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
			log.Printf("Embedded file: %s", path)
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
	VariableKeys []string // Optional; if set, these keys will be used for multi-variable input.
}

// Commands is our single authoritative list of all possible commands.
var Commands = []CommandSpec{
	{Name: "add section"}, // no template (placeholder)
	{Name: "remove section"},
	{Name: "add page", TemplatePath: "page-and-archive.json"},
	{Name: "add wordpress block", TemplatePath: "wordpress-interactive-block-for-nextgen-theme.json"},
	{Name: "add nextgen pagebuilder block", TemplatePath: "add-nextgen-pagebuilder-block.json"},
	{
		Name:         "add multiple variables example",
		TemplatePath: "multiple-variables-example.json",
		VariableKeys: []string{"ComponentName", "Page", "Feature"}, // This can be replaced with any keys you need.
	},
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
}

// RecentUsed & NextSteps remain separate slices, for usage in the UI.
var RecentUsed = []string{
	"add section",
	"remove section",
	"add wordpress block",
	"add nextgen pagebuilder block",
	"add multiple variables example",
	"add page",
	"remove page",
	"add portable-component",
	"remove portable-component",
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
	"undo": "↺",
	"redo": "↻",
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

// RunCommand checks if the command is recognized and, if it has a TemplatePath,
// fetches that JSON from embedded memory. Otherwise, it's just a placeholder.
func RunCommand(cmdName, projectPath string, placeholders map[string]string) error {
	tPath, found := TemplatePathFor(cmdName)
	if !found {
		return fmt.Errorf("unknown command: %q", cmdName)
	}

	// If TemplatePath is empty -> "not yet implemented."
	if tPath == "" {
		fmt.Printf("[Placeholder] %q command is recognized but not yet implemented.\n", cmdName)
		return nil
	}

	fmt.Printf("Running command: %s\n", cmdName)
	fmt.Printf("Template path (key in embed.FS): %s\n", tPath)
	fmt.Printf("Project path: %s\n", projectPath)
	fmt.Printf("Placeholders: %+v\n", placeholders)

	// Load template bytes from memory via the registry:
	data, err := LoadCommandTemplate(tPath)
	if err != nil {
		return fmt.Errorf("template %q not found in embedded registry: %w", tPath, err)
	}

	// Run the JSON template. (File actions have been removed.)
	return RunJsonTemplateBytes(data, projectPath, placeholders)
}
