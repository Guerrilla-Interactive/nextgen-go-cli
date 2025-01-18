package commands

import (
	"fmt"
	"os"
)

// CommandSpec defines a single command’s name and optional template path.
type CommandSpec struct {
	Name         string
	TemplatePath string // If non-empty, we run JSON. If empty, it's a placeholder.
}

// Commands is our single authoritative list of all possible commands.
var Commands = []CommandSpec{
	{Name: "add section"},    // no template (placeholder)
	{Name: "remove section"}, // no template (placeholder)
	{Name: "add page", TemplatePath: "app/commands/page-and-archive.json"},
	{Name: "remove page"}, // no template (placeholder)
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

// RecentUsed & NextSteps remain separate slices, as they’re used differently in your UI.
var RecentUsed = []string{
	"add section",
	"remove section",
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

// CommandIconMap maps commands to icons.
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
	"remove schema":             "✖",
	"add query":                 "✚",
	"remove query":              "✖",
	"add sanity-plugin":         "✚",
	"remove sanity-plugin":      "✖",
	"undo":                      "↺",
	"redo":                      "↻",
	"add hello":                 "✚",
}

// CommandWithIcon prefixes the command with its icon. Fallback is a bullet.
func CommandWithIcon(cmd string) string {
	if icon, ok := CommandIconMap[cmd]; ok {
		return fmt.Sprintf("%s  %s", icon, cmd)
	}
	return fmt.Sprintf("•  %s", cmd)
}

// AllCommandNames returns the command names in order. Useful for the “all commands” screen.
func AllCommandNames() []string {
	names := make([]string, len(Commands))
	for i, c := range Commands {
		names[i] = c.Name
	}
	return names
}

// TemplatePathFor locates the first command with the given name & returns its TemplatePath (and true if found).
func TemplatePathFor(cmdName string) (string, bool) {
	for _, c := range Commands {
		if c.Name == cmdName {
			return c.TemplatePath, true
		}
	}
	return "", false
}

// RunCommand checks if the command is recognized, then either runs a JSON template
// (if TemplatePath is non-empty) or returns a “not yet implemented” placeholder message.
func RunCommand(cmdName, projectPath string, placeholders map[string]string) error {
	tPath, found := TemplatePathFor(cmdName)
	if !found {
		return fmt.Errorf("unknown command: %q", cmdName)
	}
	if tPath == "" {
		fmt.Printf("[Placeholder] %q command is recognized but not yet implemented.\n", cmdName)
		return nil
	}

	// Add debug information
	fmt.Printf("Running command: %s\n", cmdName)
	fmt.Printf("Template path: %s\n", tPath)
	fmt.Printf("Project path: %s\n", projectPath)
	fmt.Printf("Placeholders: %+v\n", placeholders)

	// Try to read the template file first to verify it exists
	if _, err := os.Stat(tPath); os.IsNotExist(err) {
		return fmt.Errorf("template file not found at %s", tPath)
	}

	return RunJsonTemplate(tPath, projectPath, placeholders)
}
