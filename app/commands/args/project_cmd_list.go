package args

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	// Add imports for project config loading later
)

// ProjectCmdListCommand defines the command to list project-specific commands.
type ProjectCmdListCommand struct{}

func init() {
	RegisterCommand(&ProjectCmdListCommand{})
}

func (c *ProjectCmdListCommand) Name() string {
	return "project-cmd list"
}

func (c *ProjectCmdListCommand) Description() string {
	return "Lists custom commands defined for the current project."
}

func (c *ProjectCmdListCommand) Usage() string {
	return ""
}

func (c *ProjectCmdListCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

func (c *ProjectCmdListCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{}
}

func (c *ProjectCmdListCommand) Execute(args cli.CommandArgs) error {
	fmt.Println("Executing project-cmd list...")

	// Get current working directory as project path
	projectPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")

	entries, err := os.ReadDir(localCmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No project-specific commands found (directory .nextgen/local-commands does not exist).")
			return nil
		}
		return fmt.Errorf("failed to read local commands directory '%s': %w", localCmdDir, err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			names = append(names, name)
		}
	}

	if len(names) == 0 {
		fmt.Println("No project-specific commands found in .nextgen/local-commands/")
		return nil
	}

	sort.Strings(names)
	fmt.Println("Available Project Commands:")
	for _, name := range names {
		// TODO: Optionally read the JSON to get a description?
		fmt.Printf("  - %s\n", name)
	}

	return nil
}
