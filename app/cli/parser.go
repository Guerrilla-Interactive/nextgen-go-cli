package cli

import (
	"fmt"
	"strings"
)

// CommandRegistryChecker defines an interface for checking if a command name exists.
// This avoids a direct dependency cycle between cli and commands packages.
type CommandRegistryChecker interface {
	CommandExists(name string) bool
}

// ArgDef defines the structure for an expected positional argument.
type ArgDef struct {
	Name        string // e.g., "filename", "count"
	Description string // Help text for the argument
	Required    bool   // Whether the argument is mandatory
}

// FlagDef defines the structure for an expected flag.
type FlagDef struct {
	Name        string // Long name (e.g., "output")
	ShortName   string // Short name (e.g., "o"), empty if none
	Description string // Help text for the flag
	HasValue    bool   // Whether the flag expects a value (true for --flag=v, false for --flag)
	Required    bool   // Whether the flag is mandatory
}

// CommandArgs holds structured information parsed from command-line arguments.
type CommandArgs struct {
	RawArgs          []string          // Keep the original args for potential re-parsing or complex scenarios
	CommandName      string            // The command specified (e.g., "add-page", "config set")
	Variables        []string          // Positional arguments provided after the command name
	Flags            map[string]string // Flags provided (e.g., --output=./path -> map["output"]="./path")
	BoolFlags        map[string]bool   // Boolean flags (e.g., --force -> map["force"]=true)
	HelpRequested    bool              // If a help flag (--help, -h) was detected
	VersionRequested bool              // If a version flag (--version) was detected
	Errors           []error           // Any parsing errors encountered
}

// Debug toggle controlled by --debug. Other packages can query this.
var debugEnabled bool

// SetDebugEnabled enables or disables debug logging globally for this process.
func SetDebugEnabled(on bool) { debugEnabled = on }

// IsDebugEnabled reports whether debug logging is currently enabled.
func IsDebugEnabled() bool { return debugEnabled }

// Verbose toggle controlled by --verbose for informational (non-debug) output.
var verboseEnabled bool

// SetVerboseEnabled enables or disables verbose informational output globally.
func SetVerboseEnabled(on bool) { verboseEnabled = on }

// IsVerboseEnabled reports whether verbose mode is currently enabled.
func IsVerboseEnabled() bool { return verboseEnabled }

// ParseCommandLineArgs processes the raw command-line arguments using a command registry checker.
func ParseCommandLineArgs(rawArgs []string, registry CommandRegistryChecker) CommandArgs {
	parsed := CommandArgs{
		RawArgs:   rawArgs,
		Variables: make([]string, 0),
		Flags:     make(map[string]string),
		BoolFlags: make(map[string]bool),
		Errors:    make([]error, 0),
	}

	args := make([]string, len(rawArgs)) // Work on a copy
	copy(args, rawArgs)

	// --- Stage 0: Scan *raw* args for global flags first ---
	// This ensures intent is captured regardless of position
	for _, arg := range rawArgs {
		if arg == "--help" || arg == "-h" {
			parsed.HelpRequested = true
		} else if arg == "--version" {
			parsed.VersionRequested = true
		}
	}

	// --- Stage 1: Find Potential Command Name(s) ---
	firstPotentialCommandIndex := -1
	secondPotentialCommandIndex := -1
	for i, arg := range args { // Use the copy `args` here
		if !strings.HasPrefix(arg, "-") {
			if firstPotentialCommandIndex == -1 {
				firstPotentialCommandIndex = i
			} else if secondPotentialCommandIndex == -1 {
				secondPotentialCommandIndex = i
				break
			}
		}
	}

	// Determine Command Name and remaining args for flag parsing
	argsToParseFlagsFrom := args
	if firstPotentialCommandIndex != -1 {
		// Check if first two non-flag args form a valid multi-word command
		if secondPotentialCommandIndex != -1 {
			potentialCommandName := args[firstPotentialCommandIndex] + " " + args[secondPotentialCommandIndex]
			if registry.CommandExists(potentialCommandName) {
				parsed.CommandName = potentialCommandName
				// Rebuild the list of args for flag/variable parsing, excluding the command parts
				tempArgs := []string{}
				for i, arg := range args {
					if i != firstPotentialCommandIndex && i != secondPotentialCommandIndex {
						tempArgs = append(tempArgs, arg)
					}
				}
				argsToParseFlagsFrom = tempArgs
			} else {
				// Check if the first non-flag arg is a valid single-word command
				if registry.CommandExists(args[firstPotentialCommandIndex]) {
					parsed.CommandName = args[firstPotentialCommandIndex]
					argsToParseFlagsFrom = append(args[:firstPotentialCommandIndex], args[firstPotentialCommandIndex+1:]...)
				} else {
					// Treat first non-flag arg as a variable if not a command
					// Keep all args for flag/var parsing, CommandName remains empty
					argsToParseFlagsFrom = args
				}
			}
		} else {
			// Only one potential command word found
			if registry.CommandExists(args[firstPotentialCommandIndex]) {
				parsed.CommandName = args[firstPotentialCommandIndex]
				argsToParseFlagsFrom = append(args[:firstPotentialCommandIndex], args[firstPotentialCommandIndex+1:]...)
			} else {
				// Treat single non-flag arg as a variable
				argsToParseFlagsFrom = args
			}
		}
	} else {
		// No command name found (e.g., only flags like `ng --help`)
		argsToParseFlagsFrom = args
	}

	// --- Stage 2: Parse Flags and Variables from the determined args list ---
	for i := 0; i < len(argsToParseFlagsFrom); i++ {
		arg := argsToParseFlagsFrom[i]

		// Skip only the global --version flag here
		if arg == "--version" {
			continue
		}

		// Parse --help / -h like any other flag in this stage
		if strings.HasPrefix(arg, "--") {
			flagPart := strings.TrimPrefix(arg, "--")
			flagName := flagPart
			flagValue := ""
			hasExplicitValue := false

			if strings.Contains(flagPart, "=") {
				parts := strings.SplitN(flagPart, "=", 2)
				flagName = parts[0]
				flagValue = parts[1]
				hasExplicitValue = true
			} else if i+1 < len(argsToParseFlagsFrom) && !strings.HasPrefix(argsToParseFlagsFrom[i+1], "-") {
				flagValue = argsToParseFlagsFrom[i+1]
				hasExplicitValue = true
				i++ // Consume the value argument
			}

			if hasExplicitValue {
				if _, exists := parsed.Flags[flagName]; exists {
					parsed.Errors = append(parsed.Errors, fmt.Errorf("flag provided more than once: --%s", flagName))
				}
				parsed.Flags[flagName] = flagValue
			} else {
				if _, exists := parsed.BoolFlags[flagName]; exists {
					parsed.Errors = append(parsed.Errors, fmt.Errorf("boolean flag provided more than once: --%s", flagName))
				}
				parsed.BoolFlags[flagName] = true
			}
		} else if strings.HasPrefix(arg, "-") {
			flagChars := strings.TrimPrefix(arg, "-")

			if len(flagChars) == 0 {
				parsed.Errors = append(parsed.Errors, fmt.Errorf("invalid flag format: %s", arg))
				continue
			}

			potentialValue := ""
			valueConsumed := false
			if i+1 < len(argsToParseFlagsFrom) && !strings.HasPrefix(argsToParseFlagsFrom[i+1], "-") {
				potentialValue = argsToParseFlagsFrom[i+1]
			}

			for j, flagChar := range flagChars {
				flagName := string(flagChar)

				if j == len(flagChars)-1 && potentialValue != "" {
					if _, exists := parsed.Flags[flagName]; exists {
						parsed.Errors = append(parsed.Errors, fmt.Errorf("flag provided more than once: -%s", flagName))
					}
					parsed.Flags[flagName] = potentialValue
					valueConsumed = true
				} else {
					if _, exists := parsed.BoolFlags[flagName]; exists {
						parsed.Errors = append(parsed.Errors, fmt.Errorf("boolean flag provided more than once: -%s", flagName))
					}
					parsed.BoolFlags[flagName] = true
				}
			}

			if valueConsumed {
				i++
			}
		} else {
			parsed.Variables = append(parsed.Variables, arg)
		}
	}

	return parsed
}
