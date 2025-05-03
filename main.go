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
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	// Import the template commands package for helpers
	"time" // Add for history recording

	template_cmds "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils" // Import utils for file tree

	// Use alias for args package
	args_pkg "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands/args"
)

// Define Version (will be set via linker flags during build)
var Version = "v1.0.61"

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

// Init returns the Cmd that loads project info from screens.InitProjectCmd.
func (pm ProgramModel) Init() tea.Cmd {
	// This Cmd will eventually yield an Msg containing an updated app.Model (with ProjectPath set).
	return screens.InitProjectCmd(pm.M)
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
	case screens.CommandFinishedMsg:
		if typedMsg.Err != nil {
			// Optionally log or display the error.
			fmt.Println("Command finished with error:", typedMsg.Err)
		}
		// Update to installation details screen.
		pm.M.CurrentScreen = app.ScreenInstallDetails

	// 3) Handle window size message
	case tea.WindowSizeMsg:
		// Record terminal dimensions for layout purposes.
		pm.M.TerminalWidth = typedMsg.Width
		pm.M.TerminalHeight = typedMsg.Height
		return pm, nil

	case tea.KeyMsg:
		switch pm.M.CurrentScreen {
		case app.ScreenSelect:
			updatedM, cmd := screens.UpdateScreenSelect(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenMain:
			updatedM, cmd := screens.UpdateScreenMain(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenFilenamePrompt:
			updatedM, cmd := screens.UpdateScreenFilenamePrompt(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenInstallDetails:
			updatedM, cmd := screens.UpdateInstallDetailsScreen(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectStats:
			updatedM, cmd := screens.UpdateScreenProjectStats(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandHistory:
			updatedM, cmd := screens.UpdateScreenCommandHistory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandsCategory:
			updatedM, cmd := screens.UpdateScreenCommandsCategory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardList:
			updatedM, cmd := screens.UpdateScreenClipboardList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardActions:
			updatedM, cmd := screens.UpdateScreenClipboardActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenRenameClipboard:
			updatedM, cmd := screens.UpdateScreenRenameClipboard(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeList:
			// UpdateScreenNativeList *does* need the registry now (for consistency)
			updatedM, cmd := screens.UpdateScreenNativeList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeActions:
			updatedM, cmd := screens.UpdateScreenNativeActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandsList:
			updatedM, cmd := screens.UpdateScreenProjectCommandsList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandActions:
			updatedM, cmd := screens.UpdateScreenProjectCommandActions(pm.M, typedMsg, pm.ProjectRegistry)
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
		return screens.ViewSelectScreen(pm.M)
	case app.ScreenMain:
		return screens.ViewMainScreen(pm.M, pm.ProjectRegistry)
	case app.ScreenFilenamePrompt:
		return screens.ViewFilenamePrompt(pm.M, pm.ProjectRegistry)
	case app.ScreenInstallDetails:
		return screens.ViewInstallDetailsScreen(pm.M)
	case app.ScreenProjectStats:
		return screens.ViewProjectStatsScreenWithRegistry(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandHistory:
		return screens.ViewScreenCommandHistory(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandsCategory:
		return screens.ViewScreenCommandsCategory(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardList:
		return screens.ViewScreenClipboardList(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardActions:
		return screens.ViewScreenClipboardActions(pm.M, pm.ProjectRegistry)
	case app.ScreenRenameClipboard:
		return screens.ViewScreenRenameClipboard(pm.M)
	case app.ScreenNativeList:
		return screens.ViewScreenNativeList(pm.M, pm.ProjectRegistry)
	case app.ScreenNativeActions:
		return screens.ViewScreenNativeActions(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandsList:
		return screens.ViewScreenProjectCommandsList(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandActions:
		// --- Add Debug Logging Here ---
		fmt.Fprintf(os.Stderr, "DEBUG: Routing to ViewScreenProjectCommandActions\n")
		fmt.Fprintf(os.Stderr, "DEBUG:   pm.M.SelectedProjectCommand = '%s'\n", pm.M.SelectedProjectCommand)
		registryIsNil := pm.ProjectRegistry == nil
		fmt.Fprintf(os.Stderr, "DEBUG:   pm.ProjectRegistry == nil: %t\n", registryIsNil)
		// --- End Debug Logging ---
		return screens.ViewScreenProjectCommandActions(pm.M, pm.ProjectRegistry)
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

	// 1. Try executing as an Arg-based command first
	cmd, found := args_pkg.GetCommand(commandName)
	if found {
		fmt.Printf("DEBUG: Executing command '%s' via args package...\n", commandName)
		err := cmd.Execute(args)
		fmt.Printf("DEBUG: Args command '%s' finished. Error: %v\n", commandName, err)
		if err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
		return nil // Success
	}

	// 2. Try executing as a User-Saved Native Command
	if registry != nil && registry.NativeCommands != nil {
		if commandString, nativeFound := registry.NativeCommands[commandName]; nativeFound {
			fmt.Printf("DEBUG: Executing command '%s' as user-saved native command...\n", commandName)
			fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
			return runShellCommand(commandString, commandArgs, projectPath)
		}
	}

	// 3. Try executing as a Clipboard Command
	if registry != nil && registry.ClipboardCommands != nil {
		// DEBUG PRINT: List available clipboard keys
		clipboardKeys := make([]string, 0, len(registry.ClipboardCommands))
		for k := range registry.ClipboardCommands {
			clipboardKeys = append(clipboardKeys, k)
		}
		fmt.Printf("DEBUG: Available clipboard keys: %v\n", clipboardKeys)

		fmt.Printf("DEBUG: Checking clipboard commands map for key: '%s'\n", commandName) // DEBUG PRINT 1
		if clipboardSpec, clipboardFound := registry.ClipboardCommands[commandName]; clipboardFound {
			fmt.Printf("DEBUG: Clipboard command '%s' FOUND in map.\n", commandName) // DEBUG PRINT 2
			fmt.Printf("DEBUG: Executing command '%s' as clipboard command...\n", commandName)
			templateString := clipboardSpec.Template
			if templateString == "" {
				return fmt.Errorf("clipboard command '%s' has empty template content", commandName)
			}
			templateBytes := []byte(templateString)
			keys := template_cmds.InferVariableKeys(string(templateBytes))
			if len(keys) != len(commandArgs) {
				return fmt.Errorf("clipboard command '%s' requires %d argument(s) (%s), but %d provided",
					commandName, len(keys), strings.Join(keys, ", "), len(commandArgs))
			}
			varsMap := make(map[string]string)
			for i, key := range keys {
				varsMap[key] = commandArgs[i]
			}
			placeholders := template_cmds.BuildPlaceholders(varsMap)
			fmt.Printf("DEBUG: Running clipboard template with placeholders: %+v\n", placeholders)
			template_cmds.CreatedFiles = []string{}
			template_cmds.EditedIndexers = make(map[string]bool)
			execErr := template_cmds.ExecuteJSONTemplateFromMemory(templateBytes, projectPath, placeholders)
			// --- Record History ---
			fmt.Printf("DEBUG: Clipboard template executed. Generated: %v, Edited Indexers: %v\n", template_cmds.CreatedFiles, template_cmds.EditedIndexers)
			if registry != nil && projectPath != "." {
				if projectInfo, found := registry.GetProject(projectPath); found {
					historicCmd := project.HistoricCommand{
						Name:           commandName,
						Variables:      placeholders,
						Timestamp:      time.Now().Unix(),
						GeneratedFiles: append([]string{}, template_cmds.CreatedFiles...),
					}
					if projectInfo.CommandHistory == nil {
						projectInfo.CommandHistory = []project.HistoricCommand{}
					}
					projectInfo.CommandHistory = append(projectInfo.CommandHistory, historicCmd)
					if len(projectInfo.CommandHistory) > 20 {
						projectInfo.CommandHistory = projectInfo.CommandHistory[len(projectInfo.CommandHistory)-20:]
					}
					registry.AddOrUpdateProject(projectInfo)
					if saveErr := registry.Save(); saveErr != nil {
						fmt.Printf("Warning: Failed to save project registry after executing clipboard command '%s': %v\n", commandName, saveErr)
					}
				} else {
					fmt.Printf("Warning: Project '%s' not found in registry, cannot record history for clipboard command.\n", projectPath)
				}
			}
			// ----------------------
			if execErr != nil {
				return fmt.Errorf("clipboard template execution failed: %w", execErr)
			}
			// --- Print File Tree on Success ---
			if len(template_cmds.CreatedFiles) > 0 {
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
				// Render without icons or special markers for CLI output
				treeString := utils.RenderFileTree(treeRoot, "", false, false, nil)
				fmt.Println(treeString)
			}
			// ---------------------------------
			return nil // Success for clipboard command
		} else {
			fmt.Printf("DEBUG: Clipboard command '%s' NOT FOUND in map.\n", commandName) // DEBUG PRINT 3
		}
	} else {
		fmt.Println("DEBUG: Registry or ClipboardCommands map is nil, skipping check.") // DEBUG PRINT 4
	}

	// 4. Try executing as a Project Command
	localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
	kebabName := template_cmds.ToKebabCase(commandName)
	cmdFilePath := filepath.Join(localCmdDir, kebabName+".json")
	if _, err := os.Stat(cmdFilePath); err == nil {
		// File exists, try to read and execute
		jsonData, readErr := os.ReadFile(cmdFilePath)
		if readErr != nil {
			return fmt.Errorf("failed to read project command file '%s': %w", cmdFilePath, readErr)
		}

		// Reuse struct from args/project_cmd_add.go
		type ProjectCommandFile struct {
			Command string `json:"command"`
		}
		var cmdData ProjectCommandFile
		if jsonErr := json.Unmarshal(jsonData, &cmdData); jsonErr != nil {
			return fmt.Errorf("failed to parse project command file '%s': %w", cmdFilePath, jsonErr)
		}

		commandString := cmdData.Command
		if commandString == "" {
			return fmt.Errorf("command string is empty in project command file '%s'", cmdFilePath)
		}

		fmt.Printf("DEBUG: Executing command '%s' as project command...\n", commandName)
		fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
		return runShellCommand(commandString, commandArgs, projectPath)
	}

	// 5. Try executing as a Built-in Template Command
	spec := template_cmds.GetCommandSpec(commandName)
	if spec.TemplatePath != "" {
		fmt.Printf("DEBUG: Executing command '%s' as built-in template command...\n", commandName)
		// Load template bytes
		templateBytes, loadErr := template_cmds.LoadCommandTemplate(spec.TemplatePath)
		if loadErr != nil {
			return fmt.Errorf("failed to load template %s: %w", spec.TemplatePath, loadErr)
		}

		// Infer variable keys from template
		keys := template_cmds.InferVariableKeys(string(templateBytes))

		// --- Map Args to Placeholders (Simplified Approach) ---
		// Assumption: For direct execution, we map positional args to inferred keys in order.
		// We require the number of args to match the number of inferred keys.
		if len(keys) != len(commandArgs) {
			return fmt.Errorf("command '%s' requires %d argument(s) (%s), but %d provided",
				commandName, len(keys), strings.Join(keys, ", "), len(commandArgs))
		}
		varsMap := make(map[string]string)
		for i, key := range keys {
			varsMap[key] = commandArgs[i]
		}
		placeholders := template_cmds.BuildPlaceholders(varsMap)
		// ---------------------------------------------------------

		fmt.Printf("DEBUG: Running template with placeholders: %+v\n", placeholders)

		// Reset global trackers in template_cmds (important!)
		template_cmds.CreatedFiles = []string{}
		template_cmds.EditedIndexers = make(map[string]bool)

		// Execute the template logic
		execErr := template_cmds.ExecuteJSONTemplateFromMemory(templateBytes, projectPath, placeholders)

		// --- Record History (adapted from template_cmds.RunCommand) ---
		fmt.Printf("DEBUG: Template executed. Generated: %v, Edited Indexers: %v\n", template_cmds.CreatedFiles, template_cmds.EditedIndexers)
		if registry != nil && projectPath != "." { // Ensure we have registry and a valid project path
			if projectInfo, found := registry.GetProject(projectPath); found {
				historicCmd := project.HistoricCommand{
					Name:           commandName,
					Variables:      placeholders,
					Timestamp:      time.Now().Unix(),
					GeneratedFiles: append([]string{}, template_cmds.CreatedFiles...), // Copy slice
				}
				if projectInfo.CommandHistory == nil {
					projectInfo.CommandHistory = []project.HistoricCommand{}
				}
				projectInfo.CommandHistory = append(projectInfo.CommandHistory, historicCmd)
				if len(projectInfo.CommandHistory) > 20 { // Limit history
					projectInfo.CommandHistory = projectInfo.CommandHistory[len(projectInfo.CommandHistory)-20:]
				}
				registry.AddOrUpdateProject(projectInfo) // Updates usage count too
				if saveErr := registry.Save(); saveErr != nil {
					fmt.Printf("Warning: Failed to save project registry after executing template command '%s': %v\n", commandName, saveErr)
				}
			} else {
				fmt.Printf("Warning: Project '%s' not found in registry, cannot record history for template command.\n", projectPath)
			}
		}
		// -----------------------------------------------------------

		if execErr != nil {
			return fmt.Errorf("template execution failed: %w", execErr) // Return execution error after attempting history save
		}
		// --- Print File Tree on Success ---
		if len(template_cmds.CreatedFiles) > 0 {
			fmt.Println("\n--- Files Created --- ")
			relPaths := make([]string, len(template_cmds.CreatedFiles))
			for i, p := range template_cmds.CreatedFiles {
				if rel, err := filepath.Rel(projectPath, filepath.Join(projectPath, p)); err == nil { // Ensure relative to project path
					relPaths[i] = rel
				} else {
					relPaths[i] = p // Fallback to original if Rel fails
				}
			}
			treeRoot := utils.BuildFileTree(relPaths)
			// Render without icons or special markers for CLI output
			treeString := utils.RenderFileTree(treeRoot, "", false, false, nil)
			fmt.Println(treeString)
		}
		// ---------------------------------
		return nil // Success
	}

	// If not found in any category
	return fmt.Errorf("unknown or unsupported command for direct execution: %s", commandName)
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
