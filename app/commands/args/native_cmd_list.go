package args

import (
	"fmt"
	"sort"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project" // Import project package
	// Add imports for native command storage later
)

// NativeCmdListCommand defines the command to list saved native commands.
type NativeCmdListCommand struct{}

func init() {
	RegisterCommand(&NativeCmdListCommand{})
}

func (c *NativeCmdListCommand) Name() string {
	return "native-cmd list"
}

func (c *NativeCmdListCommand) Description() string {
	return "Lists saved native (shell) commands."
}

func (c *NativeCmdListCommand) Usage() string {
	return ""
}

func (c *NativeCmdListCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

func (c *NativeCmdListCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{}
}

func (c *NativeCmdListCommand) Execute(args cli.CommandArgs) error {
	fmt.Println("Executing native-cmd list...")

	// Load the registry
	registry, err := project.LoadProjectRegistry()
	if err != nil {
		return fmt.Errorf("failed to load project registry: %w", err)
	}

	if registry.NativeCommands == nil || len(registry.NativeCommands) == 0 {
		fmt.Println("No saved native commands found.")
		return nil
	}

	// Extract keys (command names) and sort them
	var names []string
	for name := range registry.NativeCommands {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Println("Available Native Commands:")
	for _, name := range names {
		// Could optionally show the command string too, maybe behind a flag?
		fmt.Printf("  - %s\n", name)
	}

	return nil
}
