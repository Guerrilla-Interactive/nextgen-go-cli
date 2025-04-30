package args

import (
	"fmt"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
)

// HelloCommand defines a simple example command.
type HelloCommand struct{}

// init registers the hello command when the package is initialized.
func init() {
	RegisterCommand(&HelloCommand{})
}

// Name returns the command's name.
func (c *HelloCommand) Name() string {
	return "hello"
}

// Description returns a brief help description.
func (c *HelloCommand) Description() string {
	return "Prints a simple greeting."
}

// Usage returns a brief usage string.
func (c *HelloCommand) Usage() string {
	// Example: return "<name> [--formal]"
	return ""
}

// ExpectedArgs returns definitions for expected positional arguments.
func (c *HelloCommand) ExpectedArgs() []ArgDef {
	// Example: return []ArgDef{{Name: "name", Description: "Your name", Required: true}}
	return []ArgDef{}
}

// ExpectedFlags returns definitions for expected flags.
func (c *HelloCommand) ExpectedFlags() []FlagDef {
	// Example: return []FlagDef{{Name: "formal", ShortName: "f", Description: "Use formal greeting", HasValue: false, Required: false}}
	return []FlagDef{}
}

// Execute runs the command logic.
func (c *HelloCommand) Execute(args cli.CommandArgs) error {
	// Example of accessing variables/flags if needed:
	// name := "World"
	// if len(args.Variables) > 0 {
	// 	 name = args.Variables[0]
	// }
	fmt.Println("Hello from the NextGen Go CLI!")
	return nil
}
