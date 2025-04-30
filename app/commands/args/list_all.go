package args

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"

	// Use alias to avoid package name conflict
	commands_pkg "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
)

// excludedCommands defines commands not typically listed in general command lists.
var excludedCommands = map[string]bool{
	"undo":                     true,
	"redo":                     true,
	"show all my commands":     true, // Navigation command in TUI
	"view project stats":       true, // Action command in TUI
	"logoutorloginplaceholder": true, // Navigation/action in TUI
	"paste from clipboard":     true, // Special handling
	// Add internal commands if they shouldn't be listed
	"internal:dump-registry": true,
	// Add TUI-only navigation/action commands if they exist in commands_pkg
	"commands": true, // The command to list commands itself
	// Also exclude list-all itself from the list
	"list-all": true,
}

// ListAllCommand defines the command to list all types of commands.
type ListAllCommand struct{}

func init() {
	// Keep debug print for registration confirmation
	// fmt.Println("DEBUG: init() called in app/commands/args/list_all.go")
	RegisterCommand(&ListAllCommand{})
}

func (c *ListAllCommand) Name() string {
	return "list-all"
}

func (c *ListAllCommand) Description() string {
	// Restore description for unified list
	return "Lists all available commands in prioritized order (like main screen)."
}

func (c *ListAllCommand) Usage() string {
	return ""
}

func (c *ListAllCommand) ExpectedArgs() []ArgDef {
	return []ArgDef{}
}

func (c *ListAllCommand) ExpectedFlags() []FlagDef {
	return []FlagDef{}
}

// Execute function replicates the logic from getPrioritizedCommandList
func (c *ListAllCommand) Execute(args cli.CommandArgs) error {
	fmt.Println("Available Commands (Prioritized):")

	// --- Load Registry ---
	registry, err := project.LoadProjectRegistry()
	if err != nil {
		// Handle case where registry might not exist yet gracefully
		fmt.Printf("Warning: Could not load project registry: %v. Some command types might be missing.\n", err)
		registry = &project.ProjectRegistry{} // Use an empty registry
	}

	// --- Get Project Path ---
	projectPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("Warning: Could not get current directory: %v. Project commands will be skipped.\n", err)
		projectPath = "" // Continue without project path if needed
	}

	// --- Build Prioritized List ---
	added := make(map[string]bool)
	var fullList []string

	// 1. Recent Commands (Top 5 from built-in list, excluding actions)
	recentLimit := 5
	count := 0
	// Access RecentUsed from the aliased commands package
	for _, cmd := range commands_pkg.RecentUsed {
		lower := strings.ToLower(cmd)
		// Check against the local excludedCommands map
		if _, ok := excludedCommands[lower]; ok {
			continue
		}
		if !added[cmd] && count < recentLimit {
			fullList = append(fullList, cmd)
			added[cmd] = true
			count++
		}
	}

	// 2. Favorite Native Commands (User-saved shell commands)
	if registry.FavoriteNativeCommands != nil {
		var favNative []string
		for cmdName := range registry.FavoriteNativeCommands {
			favNative = append(favNative, cmdName)
		}
		sort.Strings(favNative) // Sort favorites alphabetically
		for _, cmd := range favNative {
			if !added[cmd] {
				fullList = append(fullList, cmd)
				added[cmd] = true
			}
		}
	}

	// 3. Favorite Clipboard Commands
	if registry.ClipboardCommands != nil {
		var favClipboard []project.ClipboardCommandSpec
		for _, spec := range registry.ClipboardCommands {
			if spec.IsFavorite {
				favClipboard = append(favClipboard, spec)
			}
		}
		// Sort favorites by timestamp, newest first
		sort.SliceStable(favClipboard, func(i, j int) bool {
			return favClipboard[i].Timestamp > favClipboard[j].Timestamp
		})
		for _, spec := range favClipboard {
			if !added[spec.Name] {
				fullList = append(fullList, spec.Name)
				added[spec.Name] = true
			}
		}
	}

	// 4. Local Project Commands
	if projectPath != "" {
		localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
		entries, readErr := os.ReadDir(localCmdDir)
		if readErr == nil {
			var projNames []string
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					name := strings.TrimSuffix(entry.Name(), ".json")
					projNames = append(projNames, name)
				}
			}
			sort.Strings(projNames)
			for _, cmd := range projNames {
				if !added[cmd] {
					fullList = append(fullList, cmd)
					added[cmd] = true
				}
			}
		} else if !os.IsNotExist(readErr) {
			fmt.Printf("Warning: Could not read project commands directory '%s': %v\n", localCmdDir, readErr)
		}
	}

	// 5. Remaining Built-in Template Commands
	allBuiltIn := commands_pkg.AllCommandNames() // Assumes this exists in commands_pkg
	sort.Strings(allBuiltIn)
	for _, cmd := range allBuiltIn {
		lower := strings.ToLower(cmd)
		// Check exclusion again using the local map
		if !added[cmd] {
			if _, ok := excludedCommands[lower]; !ok {
				fullList = append(fullList, cmd)
				added[cmd] = true
			}
		}
	}

	// 6. Remaining Arg-based Commands (from this package)
	argCmds := GetAllCommands() // Get commands from this package's registry
	sort.Slice(argCmds, func(i, j int) bool {
		return argCmds[i].Name() < argCmds[j].Name()
	})
	for _, cmd := range argCmds {
		lower := strings.ToLower(cmd.Name())
		if !added[lower] { // Check added map using lower case name
			if _, ok := excludedCommands[lower]; !ok {
				fullList = append(fullList, cmd.Name())
				added[lower] = true // Add lower case name to added map
			}
		}
	}

	// 7. Remaining Clipboard Commands (non-favorite)
	if registry.ClipboardCommands != nil {
		var otherClipboard []string
		for name, spec := range registry.ClipboardCommands {
			if !spec.IsFavorite && !added[name] { // Also check if already added
				otherClipboard = append(otherClipboard, name)
			}
		}
		sort.Strings(otherClipboard)
		for _, cmd := range otherClipboard {
			fullList = append(fullList, cmd)
			added[cmd] = true
		}
	}

	// 8. Remaining Native Commands (User-saved, non-favorite)
	if registry.NativeCommands != nil {
		var otherNative []string
		for name := range registry.NativeCommands {
			isFavorite := false
			if registry.FavoriteNativeCommands != nil {
				_, isFavorite = registry.FavoriteNativeCommands[name]
			}
			if !isFavorite && !added[name] {
				otherNative = append(otherNative, name)
			}
		}
		sort.Strings(otherNative)
		for _, cmd := range otherNative {
			fullList = append(fullList, cmd)
			added[cmd] = true
		}
	}

	// --- Print the Final List ---
	if len(fullList) == 0 {
		fmt.Println("  (No commands found)")
	} else {
		for _, cmdName := range fullList {
			// TODO: Add favorite indicator (⭐) based on registry check?
			fmt.Printf("  - %s\n", cmdName)
		}
	}

	return nil
}
