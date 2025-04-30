package args

import (
	"fmt"
	"sort"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project" // Import project package
	// Add imports for clipboard storage later
)

// ClipboardListCommand defines the command to list clipboard items.
type ClipboardListCommand struct{}

func init() {
	RegisterCommand(&ClipboardListCommand{})
}

func (c *ClipboardListCommand) Name() string {
	return "clipboard-list"
}

func (c *ClipboardListCommand) Description() string {
	return "Lists items stored in the clipboard history."
}

func (c *ClipboardListCommand) Usage() string {
	return ""
}

func (c *ClipboardListCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

func (c *ClipboardListCommand) ExpectedFlags() []FlagDef {
	// Maybe add flags for filtering (e.g., --limit, --tag) later
	return []FlagDef{}
}

func (c *ClipboardListCommand) Execute(args cli.CommandArgs) error {
	fmt.Println("Executing clipboard-list...")

	// Load the registry
	registry, err := project.LoadProjectRegistry()
	if err != nil {
		return fmt.Errorf("failed to load project registry: %w", err)
	}

	if registry.ClipboardCommands == nil || len(registry.ClipboardCommands) == 0 {
		fmt.Println("No saved clipboard commands found.")
		return nil
	}

	// Extract keys (command names/IDs) and sort them
	var names []string
	for name := range registry.ClipboardCommands {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Println("Available Clipboard Commands:")
	for _, name := range names {
		// TODO: Optionally show preview or favorite status?
		fmt.Printf("  - %s\n", name)
	}

	return nil
}
