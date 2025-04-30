package args

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
)

// InternalDumpRegistryCommand defines a debug command to print the projects.json content.
type InternalDumpRegistryCommand struct{}

func init() {
	RegisterCommand(&InternalDumpRegistryCommand{})
}

func (c *InternalDumpRegistryCommand) Name() string {
	// Use a prefix to indicate it's for internal/debug use
	return "internal:dump-registry"
}

func (c *InternalDumpRegistryCommand) Description() string {
	return "(Internal) Prints the raw content of the projects.json registry file."
}

func (c *InternalDumpRegistryCommand) Usage() string {
	return ""
}

func (c *InternalDumpRegistryCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

func (c *InternalDumpRegistryCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{}
}

func (c *InternalDumpRegistryCommand) Execute(args cli.CommandArgs) error {
	// Use the same logic as project.getRegistryPath()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".config", "nextgen-cli")
	registryPath := filepath.Join(configDir, "projects.json") // Assuming filename is constant

	fmt.Printf("Attempting to read registry file from expected location: %s\n", registryPath)

	content, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("registry file not found at %s", registryPath)
		}
		return fmt.Errorf("failed to read registry file '%s': %w", registryPath, err)
	}

	// Print the raw content
	fmt.Println("--- Registry Content Start ---")
	fmt.Println(string(content))
	fmt.Println("--- Registry Content End ---")

	return nil
}
