package args

import (
	"fmt"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	// Add imports for actual config logic later
)

// ConfigGetCommand defines the command to get a configuration value.
type ConfigGetCommand struct{}

func init() {
	RegisterCommand(&ConfigGetCommand{})
}

func (c *ConfigGetCommand) Name() string {
	return "config get"
}

func (c *ConfigGetCommand) Description() string {
	return "Gets the value of a specific configuration key."
}

func (c *ConfigGetCommand) Usage() string {
	return "<key> [--global]"
}

func (c *ConfigGetCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{
		{Name: "key", Description: "The configuration key to get.", Required: true},
	}
}

func (c *ConfigGetCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{
		{Name: "global", ShortName: "g", Description: "Get the global configuration value instead of the project-specific one.", HasValue: false, Required: false},
	}
}

func (c *ConfigGetCommand) Execute(args cli.CommandArgs) error {
	if len(args.Variables) < 1 {
		return fmt.Errorf("missing required argument: key")
	}
	key := args.Variables[0]
	isGlobal := args.BoolFlags["global"]

	fmt.Printf("Executing config get...\n")
	fmt.Printf("  Key: %s\n", key)
	fmt.Printf("  Global: %t\n", isGlobal)

	// TODO: Implement config retrieval logic (Task #9)
	fmt.Printf("Placeholder: Value for '%s' would be displayed here.\n", key)
	return nil
}
