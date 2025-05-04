package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"encoding/json"
	"os/exec"
	"runtime"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	commands "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands/args"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// Import the template commands package for helpers
	"time" // Add for history recording

	template_cmds "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils" // Import utils for file tree

	// Use alias for args package
	args_pkg "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands/args"

	// Alias for settings package
	settingsScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/settings"

	// NEW import for history
	historyScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/history"

	// NEW import for clipboard
	clipboardScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/clipboard"

	// NEW import for native
	nativeScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/native"

	// NEW import for project commands
	projectCmdScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/project"

	// NEW import for category
	categoryScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/category"

	// NEW import for prompt
	promptScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/prompt"

	// NEW import for shared screens
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"

	// NEW import for main
	mainScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/main"
)

// Define Version (will be set via linker flags during build)
var Version = "v1.0.66"

// Add a new message type that will trigger quit after a delay.
type QuitAfterDelayMsg struct{}

// ProgramModel wraps app.Model so we can hold Update logic in one place.
type ProgramModel struct {
	M                app.Model
	ProjectRegistry  *project.ProjectRegistry // Track the project registry in the model
	InitialDetection bool                     // Track if initial project detection was performed
}

// commandRegistryCheckerBridge implements cli.CommandRegistryChecker using the commands package.
// This avoids a direct import cycle.
type commandRegistryCheckerBridge struct{}

func (b commandRegistryCheckerBridge) CommandExists(name string) bool {
	// Check args registry
	if args_pkg.CommandExists(name) {
		return true
	}
	// Check built-in template command list
	if _, found := template_cmds.TemplatePathFor(name); found {
		return true
	}

	// --- Load registry to check other types (inefficient, but necessary for now) ---
	// A better approach might involve passing the loaded registry to the parser
	// or having a more unified command lookup mechanism.
	registry, err := project.LoadProjectRegistry() // Load registry here
	if err == nil {                                // Only proceed if registry loaded successfully
		// Check user-saved Native Commands
		if registry.NativeCommands != nil {
			if _, nativeFound := registry.NativeCommands[name]; nativeFound {
				return true
			}
		}
		// Check Clipboard Commands
		if registry.ClipboardCommands != nil {
			if _, clipboardFound := registry.ClipboardCommands[name]; clipboardFound {
				return true
			}
		}
	} else {
		// Log warning if registry fails to load during check?
		fmt.Printf("DEBUG [CommandExists]: Could not load registry to check command '%s': %v\n", name, err)
	}

	// --- Check Project Commands (.nextgen/local-commands) ---
	projectPath, err := os.Getwd()
	if err == nil { // Only proceed if we can get the current directory
		localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
		kebabName := template_cmds.ToKebabCase(name) // Assume command name needs conversion
		cmdFilePath := filepath.Join(localCmdDir, kebabName+".json")
		if _, statErr := os.Stat(cmdFilePath); statErr == nil {
			// File exists, so the command is considered valid
			return true
		}
	}

	return false // Not found in any known location
}

// Init returns the Cmd that loads project info.
func (pm ProgramModel) Init() tea.Cmd {
	// Call InitProjectCmd from the shared package
	return sharedScreens.InitProjectCmd(pm.M)
}

// Update handles incoming Msgs (both from Init commands and user interaction).
func (pm ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {

	// 1) If the message is an app.Model, it's likely from InitProjectCmd:
	case app.Model:
		pm.M = typedMsg

		// If we have a project path and registry, update project usage
		// BUT only if this is not the initial detection (which was done in main())
		if pm.M.ProjectPath != "" && pm.ProjectRegistry != nil && !pm.InitialDetection {
			// Try to detect project information
			if projectInfo, found := project.DetectProject(pm.M.ProjectPath); found {
				// Update project registry with detected project
				pm.ProjectRegistry.AddOrUpdateProject(projectInfo)

				// Save the registry to persist changes
				if err := pm.ProjectRegistry.Save(); err != nil {
					// Just log the error, don't crash the app
					fmt.Printf("Error saving project registry: %v\n", err)
				}

				// Update recognized packages in the model
				pm.M.RecognizedPkgs = projectInfo.DetectedPackages
			}
		}

		// Mark initial detection as complete to allow future real updates
		pm.InitialDetection = false

		return pm, nil

	// 2) Handle the asynchronous command finished message.
	case app.CommandFinishedMsg:
		if typedMsg.Err == nil {
			// --- Command Succeeded: Record History ---
			if pm.ProjectRegistry != nil && typedMsg.ProjectPath != "" && typedMsg.ProjectPath != "." {
				historicCmd := project.HistoricCommand{
					Name:           typedMsg.CommandName,
					Variables:      typedMsg.Placeholders,
					Timestamp:      time.Now().Unix(),
					GeneratedFiles: typedMsg.GeneratedFiles,
				}
				if err := pm.ProjectRegistry.RecordCommandHistory(typedMsg.ProjectPath, historicCmd); err != nil {
					// Log error, but don't block UI
					pm.M.HistorySaveStatus = fmt.Sprintf("Error saving history: %v", err) // Update status message
					fmt.Printf("Warning: Failed to record command history for '%s': %v\n", typedMsg.CommandName, err)
				} else {
					pm.M.HistorySaveStatus = fmt.Sprintf("History saved for: %s", typedMsg.CommandName)
				}
			} else {
				// Cannot record history (no registry or invalid path)
				pm.M.HistorySaveStatus = "Could not save history (no registry or invalid path)"
			}
			// -------------------------------------------
			// Update to installation details screen on success
			pm.M.CurrentScreen = app.ScreenInstallDetails
		} else {
			// --- Command Failed ---
			// Optionally log or display the error.
			pm.M.HistorySaveStatus = fmt.Sprintf("Command '%s' failed: %v", typedMsg.CommandName, typedMsg.Err)
			fmt.Println("Command finished with error:", typedMsg.Err)
			// Optionally, stay on the current screen or go to an error screen instead of InstallDetails?
			// For now, still go to InstallDetails to show the error (it quits on key press)
			pm.M.CurrentScreen = app.ScreenInstallDetails
		}
		// Clear pending command info regardless of success/failure
		pm.M.PendingCommand = ""
		pm.M.Variables = nil
		pm.M.VariableKeys = nil
		pm.M.TempFilename = ""
		// No command needed from here, just update the model state
		return pm, nil

	// 3) Handle window size message
	case tea.WindowSizeMsg:
		// Record terminal dimensions for layout purposes.
		pm.M.TerminalWidth = typedMsg.Width
		pm.M.TerminalHeight = typedMsg.Height
		return pm, nil

	case tea.KeyMsg:
		switch pm.M.CurrentScreen {
		case app.ScreenSelect:
			// Use sharedScreens package
			updatedM, cmd := sharedScreens.UpdateScreenSelect(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenMain:
			// Use mainScreen alias
			updatedM, cmd := mainScreen.UpdateScreenMain(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenFilenamePrompt:
			// Use promptScreen alias
			updatedM, cmd := promptScreen.UpdateScreenFilenamePrompt(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenInstallDetails:
			// Use sharedScreens package for Update
			updatedM, cmd := sharedScreens.UpdateScreenInstallDetails(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenSettings:
			// Use the alias for settings package
			updatedM, cmd := settingsScreen.UpdateScreenSettings(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandHistory:
			// Use the new alias for history package
			updatedM, cmd := historyScreen.UpdateScreenCommandHistory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandsCategory:
			// Use categoryScreen alias
			updatedM, cmd := categoryScreen.UpdateScreenCommandsCategory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardList:
			// Use clipboardScreen alias
			updatedM, cmd := clipboardScreen.UpdateScreenClipboardList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardActions:
			// Use clipboardScreen alias
			updatedM, cmd := clipboardScreen.UpdateScreenClipboardActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenRenameClipboard:
			// Use clipboardScreen alias
			updatedM, cmd := clipboardScreen.UpdateScreenRenameClipboard(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeList:
			// Use nativeScreen alias
			updatedM, cmd := nativeScreen.UpdateScreenNativeList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeActions:
			// Use nativeScreen alias
			updatedM, cmd := nativeScreen.UpdateScreenNativeActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandsList:
			// Use projectCmdScreen alias
			updatedM, cmd := projectCmdScreen.UpdateScreenProjectCommandsList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandActions:
			// Use projectCmdScreen alias
			updatedM, cmd := projectCmdScreen.UpdateScreenProjectCommandActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		default:
			return pm, nil
		}
	// Add quit handling to save registry on exit
	case tea.QuitMsg:
		// Save project registry on exit if we have one
		if pm.ProjectRegistry != nil {
			if err := pm.ProjectRegistry.Save(); err != nil {
				fmt.Printf("Error saving project registry on exit: %v\n", err)
			}
		}
		return pm, nil
	}

	// For non-key messages or screens we didn't switch on, just return unchanged.
	return pm, nil
}

// View selects which screen's View function to call based on pm.M.CurrentScreen.
func (pm ProgramModel) View() string {
	switch pm.M.CurrentScreen {
	case app.ScreenSelect:
		// Use sharedScreens package
		return sharedScreens.ViewSelectScreen(pm.M)
	case app.ScreenMain:
		// Use mainScreen alias
		return mainScreen.ViewMainScreen(pm.M, pm.ProjectRegistry)
	case app.ScreenFilenamePrompt:
		return promptScreen.ViewFilenamePrompt(pm.M, pm.ProjectRegistry)
	case app.ScreenInstallDetails:
		// Use mainScreen alias for View for now (as shared only has placeholder)
		return mainScreen.ViewInstallDetailsScreen(pm.M)
	case app.ScreenSettings:
		// Use the alias for settings package
		return settingsScreen.ViewSettingsScreen(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandHistory:
		// Use the new alias for history package
		return historyScreen.ViewScreenCommandHistory(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandsCategory:
		// Use categoryScreen alias
		return categoryScreen.ViewScreenCommandsCategory(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardList:
		// Use clipboardScreen alias
		return clipboardScreen.ViewScreenClipboardList(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardActions:
		// Use clipboardScreen alias
		return clipboardScreen.ViewScreenClipboardActions(pm.M, pm.ProjectRegistry)
	case app.ScreenRenameClipboard:
		// Use clipboardScreen alias
		return clipboardScreen.ViewScreenRenameClipboard(pm.M)
	case app.ScreenNativeList:
		// Use nativeScreen alias
		return nativeScreen.ViewScreenNativeList(pm.M, pm.ProjectRegistry)
	case app.ScreenNativeActions:
		// Use nativeScreen alias
		return nativeScreen.ViewScreenNativeActions(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandsList:
		// Use projectCmdScreen alias
		return projectCmdScreen.ViewScreenProjectCommandsList(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandActions:
		// --- Add Debug Logging Here ---
		fmt.Fprintf(os.Stderr, "DEBUG: Routing to ViewScreenProjectCommandActions\n")
		fmt.Fprintf(os.Stderr, "DEBUG:   pm.M.SelectedProjectCommand = '%s'\n", pm.M.SelectedProjectCommand)
		registryIsNil := pm.ProjectRegistry == nil
		fmt.Fprintf(os.Stderr, "DEBUG:   pm.ProjectRegistry == nil: %t\n", registryIsNil)
		// --- End Debug Logging ---
		return projectCmdScreen.ViewScreenProjectCommandActions(pm.M, pm.ProjectRegistry)
	}
	return ""
}

func main() {
	fmt.Println("DEBUG: main() function started.")
	args := os.Args[1:] // Get arguments excluding program name

	// --- Load Project Registry ---
	fmt.Println("DEBUG: Attempting to load project registry...")
	projectRegistry, err := project.LoadProjectRegistry()
	if err != nil {
		fmt.Printf("DEBUG: Error loading project registry: %v\n", err)
		fmt.Printf("Warning: Could not load project registry: %v\n", err)
		// Continue with an empty registry rather than failing
		projectRegistry = &project.ProjectRegistry{
			Projects:     make(map[string]project.ProjectInfo),
			LastUsedPath: "",
			GlobalUsages: 0,
		}
	} else {
		fmt.Printf("DEBUG: Project registry loaded successfully from %s. Contains %d projects. Global usages: %d\n",
			projectRegistry.RegistryPath, len(projectRegistry.Projects), projectRegistry.GlobalUsages)
	}

	// Create the registry checker bridge
	registryChecker := commandRegistryCheckerBridge{}

	// --- Direct Command Execution Handling ---
	if len(args) > 0 {
		// Pass the registry checker to the parser
		parsedArgs := cli.ParseCommandLineArgs(args, registryChecker)

		// Handle parsing errors
		if len(parsedArgs.Errors) > 0 {
			fmt.Println("Error parsing arguments:")
			for _, err := range parsedArgs.Errors {
				fmt.Printf("  - %v\n", err)
			}
			os.Exit(1)
		}

		// Handle --version flag first, as it takes precedence
		if parsedArgs.VersionRequested {
			fmt.Printf("NextGen Go CLI %s\n", Version)
			os.Exit(0)
		}

		// Check for help *before* deciding whether to execute or show general help
		var isHelpIntent bool
		for _, arg := range parsedArgs.RawArgs { // Check raw args for --help or -h
			if arg == "--help" || arg == "-h" {
				isHelpIntent = true
				break
			}
		}

		// Now, route based on command name and help intent
		if parsedArgs.CommandName != "" {
			if isHelpIntent {
				// Command-specific help requested
				displayCommandHelp(parsedArgs.CommandName)
				os.Exit(0)
			} else {
				// Execute the command
				fmt.Printf("DEBUG: Command name '%s' recognized, proceeding to executeAndExit...\n", parsedArgs.CommandName)
				executeAndExit(parsedArgs, projectRegistry)
			}
		} else {
			fmt.Println("DEBUG: Command name NOT recognized by parser.")
			if isHelpIntent {
				// General help requested
				displayGeneralHelp()
				os.Exit(0)
			} else {
				// No command, no help, no version - invalid usage
				fmt.Println("Error: Invalid arguments or flags provided without a command name.")
				fmt.Println("Run `ng --help` for usage.")
				os.Exit(1)
			}
		}
	}

	// --- Interactive Mode Fallback ---
	// If no args were provided (or version/help/command handled above), start the TUI
	fmt.Println("No command-line arguments provided, starting interactive mode...")

	// Get current directory for project detection
	fmt.Println("DEBUG: Attempting to get current working directory...")
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("DEBUG: Error getting working directory: %v\n", err)
		fmt.Printf("Warning: Could not determine current directory: %v\n", err)
		currentDir = "" // Default to empty if unable to determine
	} else {
		fmt.Printf("DEBUG: Current working directory: %s\n", currentDir)
	}

	// Try to detect project information for the current directory
	var recognizedPkgs []string
	if currentDir != "" {
		if projectInfo, found := project.DetectProject(currentDir); found {
			// Update project registry with detected project and save
			projectRegistry.AddOrUpdateProject(projectInfo)
			recognizedPkgs = projectInfo.DetectedPackages
		}
	}

	// --- Initialize Paginators ---
	clipboardPaginator := paginator.New()
	clipboardPaginator.Type = paginator.Dots
	clipboardPaginator.PerPage = 10
	clipboardPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("•")
	clipboardPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	nativePaginator := paginator.New()
	nativePaginator.Type = paginator.Dots
	nativePaginator.PerPage = 10
	nativePaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	nativePaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	projectCommandsPaginator := paginator.New()
	projectCommandsPaginator.Type = paginator.Dots
	projectCommandsPaginator.PerPage = 10 // Or a different value?
	projectCommandsPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	projectCommandsPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	mainListPaginator := paginator.New()
	mainListPaginator.Type = paginator.Dots
	mainListPaginator.PerPage = 8 // Max 8 items per page for main list
	mainListPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	mainListPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	// Build your initial model
	initialModel := app.Model{
		IsLoggedIn:               true,
		CurrentScreen:            app.ScreenMain,
		ProjectPath:              currentDir,
		RecognizedPkgs:           recognizedPkgs,
		Version:                  Version,
		MainScreenFocus:          "list", // Default focus to the list
		ActionIndex:              0,
		SelectedIndex:            0, // Start list selection at 0
		ClipboardPaginator:       clipboardPaginator,
		NativePaginator:          nativePaginator,
		ProjectCommandsPaginator: projectCommandsPaginator,
		MainListPaginator:        mainListPaginator,
	}

	// Set default terminal dimensions so panels are anchored on first render.
	if initialModel.TerminalHeight == 0 {
		initialModel.TerminalHeight = 24
	}
	if initialModel.TerminalWidth == 0 {
		initialModel.TerminalWidth = 80
	}

	// Start the Bubble Tea program using ProgramModel as our root model.
	p := tea.NewProgram(
		ProgramModel{
			M:                initialModel,
			ProjectRegistry:  projectRegistry,
			InitialDetection: true, // Set to true to skip the first update detection
		},
		tea.WithAltScreen(),
	)
	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

// displayGeneralHelp prints the top-level help message.
func displayGeneralHelp() {
	fmt.Println("NextGen Go CLI - Help")
	fmt.Println("Usage: ng [command] [variables...] [--flags...]")
	fmt.Println("Run without arguments to enter interactive mode.")

	allCmds := commands.GetAllCommands()
	if len(allCmds) > 0 {
		fmt.Println("\nAvailable Commands:")
		sort.Slice(allCmds, func(i, j int) bool {
			return allCmds[i].Name() < allCmds[j].Name()
		})
		for _, cmd := range allCmds {
			fmt.Printf("  %-15s %s\n", cmd.Name(), cmd.Description())
		}
		fmt.Println("\nRun 'ng [command] --help' for more information on a specific command.")
	} else {
		fmt.Println("\nNo commands registered yet.")
	}
	fmt.Println("\nGlobal Flags: --help, -h, --version")
}

// displayCommandHelp displays detailed help for a specific command.
func displayCommandHelp(commandName string) {
	cmd, found := commands.GetCommand(commandName)
	if !found {
		fmt.Printf("Error: Unknown command '%s'\n", commandName)
		displayGeneralHelp() // Show general help as fallback
		return
	}

	// Display detailed help for the command
	fmt.Printf("Usage: ng %s %s\n\n", cmd.Name(), cmd.Usage())
	fmt.Printf("  %s\n", cmd.Description())

	args := cmd.ExpectedArgs()
	if len(args) > 0 {
		fmt.Println("\nArguments:")
		for _, arg := range args {
			required := ""
			if arg.Required {
				required = " (required)"
			}
			fmt.Printf("  %-15s %s%s\n", arg.Name, arg.Description, required)
		}
	}

	flags := cmd.ExpectedFlags()
	if len(flags) > 0 {
		fmt.Println("\nFlags:")
		for _, flag := range flags {
			if flag.Name == "help" || flag.ShortName == "h" {
				continue // Skip global help flags
			}
			flagUsage := "--" + flag.Name
			if flag.ShortName != "" {
				flagUsage += ", -" + flag.ShortName
			}
			if flag.HasValue {
				flagUsage += " <value>"
			}
			required := ""
			if flag.Required {
				required = " (required)"
			}
			fmt.Printf("  %-15s %s%s\n", flagUsage, flag.Description, required)
		}
	}
	fmt.Println("\nGlobal Flags: --help, -h, --version") // Also mention global flags here
}

// executeAndExit attempts to execute a command based on parsed args and exits.
func executeAndExit(parsedArgs cli.CommandArgs, registry *project.ProjectRegistry) {
	// Get current directory (needed for project context during execution)
	fmt.Println("DEBUG: Attempting to get current working directory for execution...")
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Warning: Could not determine current directory for command execution: %v\n", err)
		// Decide if commands can run without project context or exit
		// currentDir = ""
	}
	fmt.Printf("DEBUG: Current working directory for execution: %s\n", currentDir)

	// TODO: Potentially load project config based on currentDir for the command

	fmt.Printf("Attempting direct execution for command: %s\n", parsedArgs.CommandName)
	fmt.Printf("Variables: %v\n", parsedArgs.Variables)
	fmt.Printf("Flags: %v\n", parsedArgs.Flags)
	fmt.Printf("BoolFlags: %v\n", parsedArgs.BoolFlags)

	err = executeDirectCommand(parsedArgs, registry) // Pass registry
	if err != nil {
		fmt.Printf("Error executing command '%s': %v\n", parsedArgs.CommandName, err)
		os.Exit(1)
	}
	// fmt.Println("Direct command execution successful (placeholder).") // Removed misleading message
	os.Exit(0) // Exit after successful direct execution
}

// executeDirectCommand handles the core command execution.
func executeDirectCommand(args cli.CommandArgs, registry *project.ProjectRegistry) error {
	commandName := args.CommandName
	commandArgs := args.Variables // Positional args after command name

	// --- Define projectPath early so it's available in all blocks ---
	projectPath, pathErr := os.Getwd()
	if pathErr != nil {
		fmt.Printf("Warning: Could not get current directory: %v. Using '.' as fallback.\n", pathErr)
		projectPath = "."
	}
	// ------------------------------------------------------------------

	// Initialize execution error variable
	var execErr error
	// Keep track of placeholders if applicable (for history)
	var placeholders map[string]string

	// 1. Try executing as an Arg-based command first
	cmd, found := args_pkg.GetCommand(commandName)
	if found {
		fmt.Printf("DEBUG: Executing command '%s' via args package...\n", commandName)
		execErr = cmd.Execute(args)
		fmt.Printf("DEBUG: Args command '%s' finished. Error: %v\n", commandName, execErr)
		// Placeholders are not directly available from args commands for history
	} else if registry != nil && registry.NativeCommands != nil && registry.NativeCommands[commandName] != "" {
		// 2. Try executing as a User-Saved Native Command
		commandString := registry.NativeCommands[commandName]
		fmt.Printf("DEBUG: Executing command '%s' as user-saved native command...\n", commandName)
		fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
		execErr = runShellCommand(commandString, commandArgs, projectPath)
		// Placeholders are not applicable to shell commands for history
	} else if registry != nil && registry.ClipboardCommands != nil && registry.ClipboardCommands[commandName].Template != "" {
		// 3. Try executing as a Clipboard Command
		clipboardSpec := registry.ClipboardCommands[commandName]
		fmt.Printf("DEBUG: Executing command '%s' as clipboard command...\n", commandName)
		templateBytes := []byte(clipboardSpec.Template)
		keys := template_cmds.InferVariableKeys(string(templateBytes))
		if len(keys) != len(commandArgs) {
			execErr = fmt.Errorf("clipboard command '%s' requires %d argument(s) (%s), but %d provided",
				commandName, len(keys), strings.Join(keys, ", "), len(commandArgs))
		} else {
			varsMap := make(map[string]string)
			for i, key := range keys {
				varsMap[key] = commandArgs[i]
			}
			placeholders = template_cmds.BuildPlaceholders(varsMap) // Store placeholders
			fmt.Printf("DEBUG: Running clipboard template with placeholders: %+v\n", placeholders)
			template_cmds.CreatedFiles = []string{}
			template_cmds.EditedIndexers = make(map[string]bool)
			execErr = template_cmds.ExecuteJSONTemplateFromMemory(templateBytes, projectPath, placeholders)
		}
	} else if projectPath != "." { // Check project command only if we have a valid path
		// 4. Try executing as a Project Command
		localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
		kebabName := template_cmds.ToKebabCase(commandName)
		cmdFilePath := filepath.Join(localCmdDir, kebabName+".json")
		if _, err := os.Stat(cmdFilePath); err == nil {
			jsonData, readErr := os.ReadFile(cmdFilePath)
			if readErr != nil {
				execErr = fmt.Errorf("failed to read project command file '%s': %w", cmdFilePath, readErr)
			} else {
				type ProjectCommandFile struct {
					Command string `json:"command"`
				}
				var cmdData ProjectCommandFile
				if jsonErr := json.Unmarshal(jsonData, &cmdData); jsonErr != nil {
					execErr = fmt.Errorf("failed to parse project command file '%s': %w", cmdFilePath, jsonErr)
				} else {
					commandString := cmdData.Command
					if commandString == "" {
						execErr = fmt.Errorf("command string is empty in project command file '%s'", cmdFilePath)
					} else {
						fmt.Printf("DEBUG: Executing command '%s' as project command...\n", commandName)
						fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
						execErr = runShellCommand(commandString, commandArgs, projectPath)
						// Placeholders not applicable
					}
				}
			}
		}
	} else {
		// 5. Try executing as a Built-in Template Command (Only if not found above)
		spec := template_cmds.GetCommandSpec(commandName)
		if spec.TemplatePath != "" {
			fmt.Printf("DEBUG: Executing command '%s' as built-in template command...\n", commandName)
			templateBytes, loadErr := template_cmds.LoadCommandTemplate(spec.TemplatePath)
			if loadErr != nil {
				execErr = fmt.Errorf("failed to load template %s: %w", spec.TemplatePath, loadErr)
			} else {
				keys := template_cmds.InferVariableKeys(string(templateBytes))
				if len(keys) != len(commandArgs) {
					execErr = fmt.Errorf("command '%s' requires %d argument(s) (%s), but %d provided",
						commandName, len(keys), strings.Join(keys, ", "), len(commandArgs))
				} else {
					varsMap := make(map[string]string)
					for i, key := range keys {
						varsMap[key] = commandArgs[i]
					}
					placeholders = template_cmds.BuildPlaceholders(varsMap) // Store placeholders
					fmt.Printf("DEBUG: Running template with placeholders: %+v\n", placeholders)
					template_cmds.CreatedFiles = []string{}
					template_cmds.EditedIndexers = make(map[string]bool)
					execErr = template_cmds.ExecuteJSONTemplateFromMemory(templateBytes, projectPath, placeholders)
				}
			}
		} else {
			// If not found in any category
			return fmt.Errorf("unknown or unsupported command for direct execution: %s", commandName)
		}
	}

	// --- Record History (Centralized Logic) ---
	if execErr == nil { // Only record history if execution was successful
		historicCmd := project.HistoricCommand{
			Name:           commandName,
			Variables:      placeholders, // Will be nil for non-template commands, which is fine
			Timestamp:      time.Now().Unix(),
			GeneratedFiles: append([]string{}, template_cmds.CreatedFiles...), // Copy generated files (relevant for templates)
		}
		if err := registry.RecordCommandHistory(projectPath, historicCmd); err != nil {
			fmt.Printf("Warning: Failed to record command history for '%s': %v\n", commandName, err)
			// Don't return this error, as the command itself succeeded
		}
	} else {
		return execErr // Return the original execution error
	}

	// --- Print File Tree on Success (Only for Template Commands) ---
	if len(template_cmds.CreatedFiles) > 0 { // Check if files were generated
		fmt.Println("\n--- Files Created --- ")
		relPaths := make([]string, len(template_cmds.CreatedFiles))
		for i, p := range template_cmds.CreatedFiles {
			if rel, err := filepath.Rel(projectPath, filepath.Join(projectPath, p)); err == nil {
				relPaths[i] = rel
			} else {
				relPaths[i] = p
			}
		}
		treeRoot := utils.BuildFileTree(relPaths)
		treeString := utils.RenderFileTree(treeRoot, "", false, false, nil)
		fmt.Println(treeString)
	}

	return nil // Overall success
}

// Helper function to run a shell command
func runShellCommand(commandString string, commandArgs []string, workingDir string) error {
	var sysCmd *exec.Cmd
	fullCmdString := commandString
	if len(commandArgs) > 0 {
		fullCmdString += " " + strings.Join(commandArgs, " ")
	}

	if runtime.GOOS == "windows" {
		sysCmd = exec.Command("cmd", "/C", fullCmdString)
	} else {
		sysCmd = exec.Command("sh", "-c", fullCmdString)
	}

	sysCmd.Stdout = os.Stdout
	sysCmd.Stderr = os.Stderr
	sysCmd.Dir = workingDir // Set working directory

	if err := sysCmd.Run(); err != nil {
		// Return a more specific error including the command that failed
		return fmt.Errorf("shell command [%s] failed: %w", strings.Split(fullCmdString, " ")[0], err)
	}
	return nil // Success
}
