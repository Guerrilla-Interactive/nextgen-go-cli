package commands

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
)

// Use *.json to embed JSON files in the same directory (non-recursively).
//
// If you want to embed subfolders, you can saygo:embed **/*.json
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

// CommandSpec defines a single command's name and (optionally) a JSON template path.
// If TemplatePath is empty, the command is recognized but not yet implemented.
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

// CommandIconMap associates a command with an icon or fallback bullet.
var CommandIconMap = map[string]string{
	"add section":               "✚",
	"remove section":            "✖",
	"add page":                  "✚",
	"remove page":               "✖",
	"add portable-component":    "✚",
	"remove portable-component": "✖",
	"add component":             "✚",
	"remove component":          "✖",
	"add schema":                "✚",
	"add wordpress block":       "✚",
	"remove schema":             "✖",
	"add query":                 "✚",
	"remove query":              "✖",
	"add sanity-plugin":         "✚",
	"remove sanity-plugin":      "✖",
	"undo":                      "↺",
	"redo":                      "↻",
	"add hello":                 "✚",
}

// CommandWithIcon returns a user-friendly label with an icon prefix.
func CommandWithIcon(cmd string) string {
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

	// This next function is defined in command-helpers.go — it parses JSON in-memory:
	return RunJsonTemplateBytes(data, projectPath, placeholders)
}
