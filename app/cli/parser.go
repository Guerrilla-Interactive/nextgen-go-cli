package cli

import (
	"fmt"
	"strings"
)

// CommandArgs holds structured information parsed from command-line arguments.
type CommandArgs struct {
	CommandName      string            // The command specified (e.g., "add-page", "clipboard-paste")
	Variables        []string          // Positional arguments provided after the command name
	Flags            map[string]string // Flags provided (e.g., --output=./path -> map["output"]="./path")
	BoolFlags        map[string]bool   // Boolean flags (e.g., --force -> map["force"]=true)
	HelpRequested    bool              // If a help flag (--help, -h) was detected
	VersionRequested bool              // If a version flag (--version) was detected
	Errors           []error           // Any parsing errors encountered
}

// ParseCommandLineArgs processes the raw command-line arguments (excluding the program name itself)
// and extracts the command name, variables, and flags. It performs basic validation.
func ParseCommandLineArgs(args []string) CommandArgs {
	parsed := CommandArgs{
		Variables: make([]string, 0),
		Flags:     make(map[string]string),
		BoolFlags: make(map[string]bool),
		Errors:    make([]error, 0),
	}

	if len(args) == 0 {
		// No command or flags provided. The main app might default to interactive mode.
		return parsed
	}

	// --- Stage 1: Initial Scan for Special Flags & Command Name ---
	potentialCommandIndex := -1
	for i, arg := range args {
		if arg == "--help" || arg == "-h" {
			parsed.HelpRequested = true
			// If help is requested, we might not need to parse further, depending on desired behavior.
			// For now, we'll parse everything anyway.
		} else if arg == "--version" {
			parsed.VersionRequested = true
			// Similar to help, often execution stops here.
		}
		if potentialCommandIndex == -1 && !strings.HasPrefix(arg, "-") {
			potentialCommandIndex = i
		}
	}

	// If help or version requested, maybe return early?
	// if parsed.HelpRequested || parsed.VersionRequested {
	// 	 return parsed
	// }

	// Extract command name if found
	if potentialCommandIndex != -1 {
		parsed.CommandName = args[potentialCommandIndex]
		// Remove command name from args for further processing
		args = append(args[:potentialCommandIndex], args[potentialCommandIndex+1:]...)
	} else {
		// No command name found (only flags or empty args after filtering special flags)
		// The main app will need to handle this (e.g., show default help or error)
	}

	// --- Stage 2: Parse Remaining Args into Flags and Variables ---
	i := 0
	for i < len(args) {
		arg := args[i]

		// Skip special flags we already scanned for (could optimize by removing them earlier)
		if arg == "--help" || arg == "-h" || arg == "--version" {
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") {
			flagPart := strings.TrimPrefix(arg, "--")
			flagName := flagPart
			flagValue := "" // Explicit value required unless assigned later
			hasExplicitValue := false

			// Check for --flag=value format
			if strings.Contains(flagPart, "=") {
				parts := strings.SplitN(flagPart, "=", 2)
				flagName = parts[0]
				flagValue = parts[1]
				hasExplicitValue = true
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// Check for --flag value format
				// Note: This is ambiguous. Is `value` the flag's value or a positional arg?
				// Many libraries require `=` for values to avoid this.
				// Let's assume for now: if the next arg isn't a flag, it's the value.
				flagValue = args[i+1]
				hasExplicitValue = true
				i++ // Consume the value argument
			}

			if hasExplicitValue {
				if _, exists := parsed.Flags[flagName]; exists {
					parsed.Errors = append(parsed.Errors, fmt.Errorf("flag provided more than once: --%s", flagName))
				}
				parsed.Flags[flagName] = flagValue
			} else {
				// Boolean flag (--flag)
				if _, exists := parsed.BoolFlags[flagName]; exists {
					parsed.Errors = append(parsed.Errors, fmt.Errorf("boolean flag provided more than once: --%s", flagName))
				}
				parsed.BoolFlags[flagName] = true
			}
		} else if strings.HasPrefix(arg, "-") {
			// Handle short flags (-f, potentially combined like -abc)
			flagChars := strings.TrimPrefix(arg, "-")

			if len(flagChars) == 0 {
				parsed.Errors = append(parsed.Errors, fmt.Errorf("invalid flag format: %s", arg))
				i++
				continue
			}

			// Check if the next argument could be a value for the *last* short flag
			potentialValue := ""
			valueConsumed := false
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				potentialValue = args[i+1]
			}

			// Iterate through combined chars (e.g., "abc" in -abc)
			for j, flagChar := range flagChars {
				flagName := string(flagChar)

				// Is this the last char and is there a potential value?
				if j == len(flagChars)-1 && potentialValue != "" {
					// Assume value belongs to the last flag
					if _, exists := parsed.Flags[flagName]; exists {
						parsed.Errors = append(parsed.Errors, fmt.Errorf("flag provided more than once: -%s", flagName))
					}
					parsed.Flags[flagName] = potentialValue
					valueConsumed = true
				} else {
					// Treat as boolean flag
					if _, exists := parsed.BoolFlags[flagName]; exists {
						parsed.Errors = append(parsed.Errors, fmt.Errorf("boolean flag provided more than once: -%s", flagName))
					}
					parsed.BoolFlags[flagName] = true
				}
			}

			// Consume the value argument if used
			if valueConsumed {
				i++
			}
		} else {
			// Argument is not a flag, treat it as a variable
			parsed.Variables = append(parsed.Variables, arg)
		}
		i++
	}

	// Basic Validation (Example: Check if required flags for a known command are missing - needs command spec)
	// This level of validation is often better handled *after* parsing, based on the specific command.
	// if parsed.CommandName == "some-command" {
	// 	if _, ok := parsed.Flags["required-flag"]; !ok {
	// 		 parsed.Errors = append(parsed.Errors, fmt.Errorf("missing required flag --required-flag for command %s", parsed.CommandName))
	// 	 }
	// }

	return parsed
}
