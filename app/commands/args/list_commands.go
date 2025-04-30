package args

import (
	"fmt"
	"sort"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
)

// ListCommandsCommand defines the command to list all registered commands.
type ListCommandsCommand struct{}

// init registers the list-commands command when the package is initialized.
func init() {
	RegisterCommand(&ListCommandsCommand{})
}

// Name returns the command's name.
func (c *ListCommandsCommand) Name() string {
	return "commands"
}

// Description returns a brief help description.
func (c *ListCommandsCommand) Description() string {
	return "Lists all available commands."
}

// Usage returns a brief usage string.
func (c *ListCommandsCommand) Usage() string {
	return ""
}

// ExpectedArgs returns definitions for expected positional arguments.
func (c *ListCommandsCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

// ExpectedFlags returns definitions for expected flags.
func (c *ListCommandsCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{}
}

// Execute runs the command logic to list all commands.
func (c *ListCommandsCommand) Execute(args cli.CommandArgs) error {
	allCmds := GetAllCommands()

	if len(allCmds) == 0 {
		fmt.Println("No commands registered yet.")
		return nil
	}

	fmt.Println("Available Commands:")
	// Sort commands by name for consistent output
	sort.Slice(allCmds, func(i, j int) bool {
		return allCmds[i].Name() < allCmds[j].Name()
	})

	for _, cmd := range allCmds {
		// Don't list the 'commands' command itself in the output of 'ng commands'
		if cmd.Name() == c.Name() {
			continue
		}
		fmt.Printf("  %-15s %s\n", cmd.Name(), cmd.Description())
	}

	fmt.Println("\nRun 'ng [command] --help' for more information on a specific command.")
	return nil
}
