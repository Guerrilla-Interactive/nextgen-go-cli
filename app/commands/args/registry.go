package args

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	commands_pkg "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
)

// ArgDef is an alias for cli.ArgDef
// It replaces the local struct definition.
type ArgDef = cli.ArgDef

// FlagDef is an alias for cli.FlagDef
// It replaces the local struct definition.
type FlagDef = cli.FlagDef

// Command represents a CLI command that can be executed directly.
type Command interface {
	// Name returns the command's name (e.g., "add-page").
	Name() string
	// Description returns a brief help description for the command.
	Description() string
	// Execute runs the command logic with the parsed arguments.
	Execute(args cli.CommandArgs) error
	// Usage returns a brief usage string (e.g., "<filename> [options]").
	Usage() string
	// ExpectedArgs returns definitions for expected positional arguments.
	ExpectedArgs() []ArgDef
	// ExpectedFlags returns definitions for expected flags.
	ExpectedFlags() []FlagDef
	// TODO: Add methods for defining expected flags and variables for validation
	// and more detailed help text generation (related to Task #8).
}

// commandRegistry holds all registered CLI commands.
// We use a map for quick lookup by command name.
var commandRegistry = make(map[string]Command)

// RegisterCommand adds a command to the registry.
// It should ideally be called during initialization (e.g., in an init() function
// within each command's file).
func RegisterCommand(cmd Command) {
	if _, exists := commandRegistry[cmd.Name()]; exists {
		// Handle duplicate command registration - maybe panic or log warning?
		panic(fmt.Sprintf("Command already registered: %s", cmd.Name()))
	}
	commandRegistry[cmd.Name()] = cmd
	fmt.Printf("DEBUG: Registered command: %s\n", cmd.Name()) // Added debug print
}

// GetCommand retrieves a command from the registry by its name.
func GetCommand(name string) (Command, bool) {
	cmd, found := commandRegistry[name]
	return cmd, found
}

// CommandExists checks if a command with the given name is registered.
func CommandExists(name string) bool {
	_, found := commandRegistry[name]
	return found
}

// GetAllCommands returns a slice of all registered commands.
// Useful for generating help text.
func GetAllCommands() []Command {
	cmds := make([]Command, 0, len(commandRegistry))
	for _, cmd := range commandRegistry {
		cmds = append(cmds, cmd)
	}
	// TODO: Consider sorting the commands alphabetically for consistent help output.
	return cmds
}

// BuildAllAvailableCommandNames returns a de-duplicated, sorted list of all available
// command names across built-in templates, arg-based commands, local project commands,
// clipboard commands and user-saved native commands.
func BuildAllAvailableCommandNames(projectPath string, registry *project.ProjectRegistry) []string {
	added := make(map[string]bool)
	var all []string

	// 1) Built-in template commands
	builtIn := commands_pkg.AllCommandNames()
	for _, n := range builtIn {
		if !added[n] {
			all = append(all, n)
			added[n] = true
		}
	}

	// 2) Arg-based commands (from this package)
	argCmds := GetAllCommands()
	for _, c := range argCmds {
		name := c.Name()
		if !added[name] {
			all = append(all, name)
			added[name] = true
		}
	}

	// 3) Local project commands (.nextgen/local-commands/*.json)
	if projectPath != "" {
		localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
		if entries, err := os.ReadDir(localCmdDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
					name := strings.TrimSuffix(e.Name(), ".json")
					if !added[name] {
						all = append(all, name)
						added[name] = true
					}
				}
			}
		}
	}

	// 4) Clipboard commands (from registry)
	if registry != nil && registry.ClipboardCommands != nil {
		for name := range registry.ClipboardCommands {
			if !added[name] {
				all = append(all, name)
				added[name] = true
			}
		}
	}

	// 5) Native commands (from registry)
	if registry != nil && registry.NativeCommands != nil {
		for name := range registry.NativeCommands {
			if !added[name] {
				all = append(all, name)
				added[name] = true
			}
		}
	}

	sort.Strings(all)
	return all
}

var ansiRegexp = regexp.MustCompile("\u001B\\[[0-9;?]*[ -/]*[@-~]")

func stripANSI(s string) string { return ansiRegexp.ReplaceAllString(s, "") }

// WriteNextgenCommandsMDC writes the auto-generated list to .nextgen/nextgen-cli-commands.mdc
func WriteNextgenCommandsMDC(projectPath string, registry *project.ProjectRegistry) error {
	names := BuildAllAvailableCommandNames(projectPath, registry)

	// Ensure folder exists
	targetDir := filepath.Join(projectPath, ".nextgen")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create .nextgen directory: %w", err)
	}
	targetFile := filepath.Join(targetDir, "nextgen-cli-commands.mdc")

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Auto-generated list of NextGen CLI commands\n")
	b.WriteString("globs:\n")
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	b.WriteString("### All Commands\n")
	b.WriteString(fmt.Sprintf("Updated: %s UTC\n\n", time.Now().UTC().Format(time.RFC3339)))
	for _, n := range names {
		b.WriteString("- ")
		b.WriteString(n)
		b.WriteString("\n")
	}

	// Include file tree previews similar to UI preview logic
	b.WriteString("\n### File Tree Previews\n\n")
	for _, name := range names {
		preview := ""
		// Determine placeholders via key inference
		keys, _ := commands_pkg.GetCommandVariableKeys(name, projectPath, registry)
		var placeholderMap map[string]string
		if len(keys) > 0 {
			// Use the first key as the primary placeholder for preview
			placeholders := map[string]string{keys[0]: "<" + keys[0] + ">"}
			placeholderMap = commands_pkg.BuildPlaceholders(placeholders)
		} else {
			// Fallback if no keys found
			placeholderMap = commands_pkg.BuildAutoPlaceholders(map[string]string{"Main": "<Filename>"})
		}

		// Clipboard template
		if registry != nil {
			if clipSpec, ok := registry.ClipboardCommands[name]; ok {
				if pv, err := commands_pkg.GeneratePreviewFileTreeFromBytes([]byte(clipSpec.Template), placeholderMap, projectPath); err == nil && strings.TrimSpace(pv) != "" {
					preview = pv
				}
			}
		}

		// Project-local template
		if preview == "" && projectPath != "" {
			kebab := commands_pkg.ToKebabCase(name)
			localPath := filepath.Join(projectPath, ".nextgen", "local-commands", kebab+".json")
			if data, readErr := os.ReadFile(localPath); readErr == nil {
				if pv, err := commands_pkg.GeneratePreviewFileTreeFromBytes(data, placeholderMap, projectPath); err == nil && strings.TrimSpace(pv) != "" {
					preview = pv
				}
			}
		}

		// Built-in template
		if preview == "" {
			if pv, err := commands_pkg.GeneratePreviewFileTree(name, placeholderMap, projectPath); err == nil && strings.TrimSpace(pv) != "" {
				preview = pv
			}
		}

		b.WriteString("#### ")
		b.WriteString(name)
		b.WriteString("\n\n")

		// Usage details
		if len(keys) > 0 {
			b.WriteString("- Usage (name): ")
			b.WriteString("`ng ")
			b.WriteString(name)
			for _, k := range keys {
				b.WriteString(" <")
				b.WriteString(k)
				b.WriteString(">")
			}
			b.WriteString("`\n")
			// Slug variant if available
			if spec := commands_pkg.GetCommandSpec(name); spec.Slug != "" {
				b.WriteString("- Usage (slug): ")
				b.WriteString("`ng ")
				b.WriteString(spec.Slug)
				for _, k := range keys {
					b.WriteString(" <")
					b.WriteString(k)
					b.WriteString(">")
				}
				b.WriteString("`\n")
			}
			b.WriteString("- Variables: ")
			b.WriteString(strings.Join(keys, ", "))
			b.WriteString("\n\n")
		} else {
			// Arg-based or no variables
			b.WriteString("- Usage: ")
			b.WriteString("`ng ")
			b.WriteString(name)
			b.WriteString("`\n\n")
		}

		clean := stripANSI(preview)
		if strings.TrimSpace(clean) == "" {
			b.WriteString("(No file changes preview available)\n\n")
		} else {
			b.WriteString("```text\n")
			b.WriteString(clean)
			b.WriteString("\n```\n\n")
		}
	}

	if err := os.WriteFile(targetFile, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", targetFile, err)
	}
	return nil
}
