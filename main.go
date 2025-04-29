package main

import (
	"fmt"
	"os"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Define Version (will be set via linker flags during build)
var Version = "v1.0.53"

// Add a new message type that will trigger quit after a delay.
type QuitAfterDelayMsg struct{}

// ProgramModel wraps app.Model so we can hold Update logic in one place.
type ProgramModel struct {
	M                app.Model
	ProjectRegistry  *project.ProjectRegistry // Track the project registry in the model
	InitialDetection bool                     // Track if initial project detection was performed
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
		return screens.ViewFilenamePrompt(pm.M)
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

	// --- Direct Command Execution Handling ---
	if len(args) > 0 {
		parsedArgs := cli.ParseCommandLineArgs(args)

		// Handle parsing errors
		if len(parsedArgs.Errors) > 0 {
			fmt.Println("Error parsing arguments:")
			for _, err := range parsedArgs.Errors {
				fmt.Printf("  - %v\n", err)
			}
			os.Exit(1)
		}

		// Handle --version flag
		if parsedArgs.VersionRequested {
			fmt.Printf("NextGen Go CLI %s\n", Version)
			os.Exit(0)
		}

		// Handle --help flag (basic version)
		if parsedArgs.HelpRequested {
			if parsedArgs.CommandName != "" {
				// TODO: Implement help text generation for specific commands
				fmt.Printf("Help requested for command: %s\n", parsedArgs.CommandName)
				fmt.Println("Usage: ng [command] [variables...] [--flags...]")
				fmt.Println("Detailed command help not yet implemented.")
			} else {
				// TODO: Implement general help text generation (list commands)
				fmt.Println("NextGen Go CLI - Help")
				fmt.Println("Usage: ng [command] [variables...] [--flags...]")
				fmt.Println("Run without arguments to enter interactive mode.")
				fmt.Println("Available commands: (listing not yet implemented)")
				fmt.Println("Flags: --help, -h, --version")
			}
			os.Exit(0)
		}

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

		// Detect project if we have a current directory
		if currentDir != "" {
			if projectInfo, found := project.DetectProject(currentDir); found {
				// Update project registry with usage
				projectRegistry.AddOrUpdateProject(projectInfo)
				// Save changes
				if err := projectRegistry.Save(); err != nil {
					fmt.Printf("Warning: Could not save project registry: %v\n", err)
				}
			}
		}

		// Attempt Direct Command Execution if a command name was parsed
		if parsedArgs.CommandName != "" {
			fmt.Printf("Attempting direct execution for command: %s\n", parsedArgs.CommandName)
			fmt.Printf("Variables: %v\n", parsedArgs.Variables)
			fmt.Printf("Flags: %v\n", parsedArgs.Flags)
			fmt.Printf("BoolFlags: %v\n", parsedArgs.BoolFlags)

			// --- TODO: Task #6 Integration Point ---
			// 1. Resolve the command spec based on parsedArgs.CommandName
			// 2. Map parsedArgs.Variables and parsedArgs.Flags to the command spec's expected variables
			// 3. Execute the command directly using the core execution logic
			// 4. Display results (e.g., file tree, success/error message)
			// Example placeholder:
			err := executeDirectCommand(parsedArgs)
			if err != nil {
				fmt.Printf("Error executing command directly: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Direct command execution successful (placeholder).")
			// --- End TODO ---
			os.Exit(0) // Exit after successful direct execution
		} else {
			// No command name provided, but flags were given (e.g., just `ng --someflag`)
			// Decide how to handle this - show error? Show help? Enter interactive?
			fmt.Println("Error: Flags provided without a command name.")
			fmt.Println("Run `ng --help` for usage.")
			os.Exit(1)
		}
	}

	// --- Interactive Mode Fallback ---
	// If no args were provided (or handled above), start the TUI
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

func executeDirectCommand(args cli.CommandArgs) error {
	// TODO: Implement direct command execution logic
	return nil
}
