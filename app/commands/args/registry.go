package args

import (
	"fmt"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
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
