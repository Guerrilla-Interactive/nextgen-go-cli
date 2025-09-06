package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
	template_cmds "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands" // template helpers
	args_pkg "github.com/Guerrilla-Interactive/nextgen-go-cli/app/commands/args" // args-based commands
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
	"github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils" // file tree utils

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

	// screens
	categoryScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/category"
	clipboardScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/clipboard"
	nativeScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/native"
	projectCmdScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/commands/project"
	historyScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/history"
	loginScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/login"
	mainScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/main"
	promptScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/prompt"
	settingsScreen "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/settings"
	sharedScreens "github.com/Guerrilla-Interactive/nextgen-go-cli/app/screens/shared"
	config "github.com/Guerrilla-Interactive/nextgen-go-cli/internal"
)

// Define Version (will be set via linker flags during build)
var Version = "v1.0.111"

// exitLog holds a log-style summary printed after the TUI exits.
var exitLog string

// determine CLI name variants (primary + aliases)
func detectPrimaryCLIName() string {
	exe := filepath.Base(os.Args[0])
	lower := strings.ToLower(exe)
	if strings.Contains(lower, "nextgen") {
		return "nextgen"
	}
	return "ng"
}

func cliNameVariants() (string, []string) {
	primary := detectPrimaryCLIName()
	if primary == "ng" {
		return primary, []string{"ng", "nextgen"}
	}
	return primary, []string{"nextgen", "ng"}
}

func formatUsageBoth(cmdName, argSig string) string {
	_, variants := cliNameVariants()
	var b strings.Builder
	for i, n := range variants {
		b.WriteString(n)
		if strings.TrimSpace(cmdName) != "" {
			b.WriteString(" ")
			b.WriteString(cmdName)
		}
		if strings.TrimSpace(argSig) != "" {
			b.WriteString(" ")
			b.WriteString(argSig)
		}
		if i < len(variants)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Add a new message type that will trigger quit after a delay.
type QuitAfterDelayMsg struct{}

// ProgramModel wraps app.Model so we can hold Update logic in one place.
type ProgramModel struct {
	M                app.Model
	ProjectRegistry  *project.ProjectRegistry // Track the project registry in the model
	InitialDetection bool                     // Track if initial project detection was performed
}

// projectCommandFile represents a local project command that wraps a shell command string.
type projectCommandFile struct {
	Command string `json:"command"`
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
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG [CommandExists]: Could not load registry to check command '%s': %v\n", name, err)
		}
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

type HeartbeatMsg struct{}

func heartbeatTick() tea.Cmd {
    return tea.Tick(time.Millisecond*800, func(time.Time) tea.Msg { return HeartbeatMsg{} })
}

// --- Helpers to build a log-style tree for exit output ---
type logNode struct {
    name     string
    path     string
    isFile   bool
    children map[string]*logNode
}

func (n *logNode) addChild(name string, isFile bool) *logNode {
    if n.children == nil {
        n.children = make(map[string]*logNode)
    }
    if child, ok := n.children[name]; ok {
        return child
    }
    child := &logNode{name: name, isFile: isFile}
    n.children[name] = child
    return child
}

func buildLogTree(paths []string) *logNode {
    root := &logNode{name: "", children: make(map[string]*logNode)}
    for _, p := range paths {
        norm := filepath.ToSlash(p)
        parts := strings.Split(norm, "/")
        cur := root
        for i, part := range parts {
            isFile := i == len(parts)-1
            child := cur.addChild(part, isFile)
            if isFile {
                child.path = p
            }
            cur = child
        }
    }
    return root
}

func renderLogTree(node *logNode, prefix string, isLast bool, skipSelf bool) string {
    var line string
    if !skipSelf && node.name != "" {
        branch := "┣"
        if isLast {
            branch = "┗"
        }
        icon := "📜"
        if len(node.children) > 0 {
            icon = "📂"
        }
        nameOut := node.name
        if node.isFile {
            if edited, ok := template_cmds.EditedIndexers[node.path]; ok && edited {
                nameOut += " (edited)"
            }
        }
        line = fmt.Sprintf("%s%s %s %s\n", prefix, branch, icon, nameOut)
    }
    newPrefix := prefix
    if node.name != "" {
        if isLast {
            newPrefix += "   "
        } else {
            newPrefix += "┃  "
        }
    }
    // sort children
    var names []string
    for name := range node.children {
        names = append(names, name)
    }
    sort.Strings(names)
    var out = line
    for i, name := range names {
        child := node.children[name]
        last := i == len(names)-1
        out += renderLogTree(child, newPrefix, last, false)
    }
    return out
}

func buildExitLog(msg app.CommandFinishedMsg) string {
    var b strings.Builder
    b.WriteString(strings.Repeat("─", 48))
    b.WriteString("\n")
    if msg.Err == nil {
        b.WriteString("Installation Complete! ✅\n")
    } else {
        b.WriteString(fmt.Sprintf("Command failed: %v\n", msg.Err))
    }
    if strings.TrimSpace(msg.ProjectPath) != "" {
        b.WriteString(msg.ProjectPath)
        b.WriteString("\n\n")
    }
    // Make paths relative to project when possible
    var rels []string
    for _, full := range msg.GeneratedFiles {
        if msg.ProjectPath != "" {
            if rel, err := filepath.Rel(msg.ProjectPath, full); err == nil {
                rels = append(rels, rel)
                continue
            }
        }
        rels = append(rels, full)
    }
    if len(rels) > 0 {
        root := buildLogTree(rels)
        // top-level entries
        var top []string
        for name := range root.children {
            top = append(top, name)
        }
        sort.Strings(top)
        for _, name := range top {
            child := root.children[name]
            icon := "📜"
            if len(child.children) > 0 {
                icon = "📦"
            }
            nameOut := name
            if child.isFile {
                if edited, ok := template_cmds.EditedIndexers[child.path]; ok && edited {
                    nameOut += " (edited)"
                }
            }
            b.WriteString(fmt.Sprintf("%s%s\n", icon, nameOut))
            b.WriteString(renderLogTree(child, " ", true, true))
        }
    } else {
        b.WriteString("(No files generated)\n")
    }
    return b.String()
}

// Init returns the Cmd that loads project info.
func (pm ProgramModel) Init() tea.Cmd {
	// Call InitProjectCmd from the shared package and start heartbeat
	return tea.Batch(
		sharedScreens.InitProjectCmd(pm.M),
		heartbeatTick(),
	)
}

// Update handles incoming Msgs (both from Init commands and user interaction).
func (pm ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case loginScreen.LoginCompletedMsg:
		updated, cmd := loginScreen.HandleLoginMsg(pm.M, typedMsg)
		pm.M = updated
		return pm, cmd

	case loginScreen.FetchUserCompletedMsg:
		updated, cmd := loginScreen.HandleLoginMsg(pm.M, typedMsg)
		pm.M = updated
		return pm, cmd

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
                    pm.M.HistorySaveStatus = fmt.Sprintf("Error saving history: %v", err)
                } else {
                    pm.M.HistorySaveStatus = fmt.Sprintf("History saved for: %s", typedMsg.CommandName)
                }
            } else {
                pm.M.HistorySaveStatus = "Could not save history (no registry or invalid path)"
            }
        } else {
            // --- Command Failed ---
            pm.M.HistorySaveStatus = fmt.Sprintf("Command '%s' failed: %v", typedMsg.CommandName, typedMsg.Err)
        }
        // Prepare an exit log and quit the TUI, printing after exit
        exitLog = buildExitLog(typedMsg)
        // Clear pending command info
        pm.M.PendingCommand = ""
        pm.M.Variables = nil
        pm.M.VariableKeys = nil
        pm.M.TempFilename = ""
        return pm, tea.Quit

	// 3) Handle window size message
	case tea.WindowSizeMsg:
		// Record terminal dimensions for layout purposes.
		pm.M.TerminalWidth = typedMsg.Width
		pm.M.TerminalHeight = typedMsg.Height
		return pm, nil

	case HeartbeatMsg:
		// Auto-start login flow when entering the Login screen (once)
		if pm.M.CurrentScreen == app.ScreenLogin && !pm.M.IsLoggedIn && !pm.M.LoginFlowStarted {
			pm.M.LoginFlowStarted = true
			return pm, tea.Batch(loginScreen.StartLoginFlowCmd(), heartbeatTick())
		}
		// Periodic driver to allow screens to refresh state (e.g., clipboard preview)
		if pm.M.CurrentScreen == app.ScreenMain {
			updatedM, cmd := mainScreen.UpdateScreenMain(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, tea.Batch(cmd, heartbeatTick())
		}
		return pm, heartbeatTick()

	case promptScreen.PromptPreviewMsg:
		updatedM, cmd := promptScreen.UpdateScreenFilenamePromptPreview(pm.M, typedMsg, pm.ProjectRegistry)
		pm.M = updatedM
		return pm, cmd

	case tea.KeyMsg:
		// Global: Ctrl+C always quits regardless of current screen
		if typedMsg.String() == "ctrl+c" {
			return pm, tea.Quit
		}
		switch pm.M.CurrentScreen {
		case app.ScreenSelect:
			updatedM, cmd := sharedScreens.UpdateScreenSelect(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenMain:
			updatedM, cmd := mainScreen.UpdateScreenMain(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenLogin:
			updatedM, cmd := loginScreen.UpdateScreenLogin(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenFilenamePrompt:
			updatedM, cmd := promptScreen.UpdateScreenFilenamePrompt(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenChoicePrompt:
			updatedM, cmd := promptScreen.UpdateScreenChoicePrompt(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenInstallDetails:
			updatedM, cmd := mainScreen.UpdateInstallDetailsScreen(pm.M, typedMsg)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenSettings:
			updatedM, cmd := settingsScreen.UpdateScreenSettings(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandHistory:
			updatedM, cmd := historyScreen.UpdateScreenCommandHistory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenCommandsCategory:
			updatedM, cmd := categoryScreen.UpdateScreenCommandsCategory(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardList:
			updatedM, cmd := clipboardScreen.UpdateScreenClipboardList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenClipboardActions:
			updatedM, cmd := clipboardScreen.UpdateScreenClipboardActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenRenameClipboard:
			updatedM, cmd := clipboardScreen.UpdateScreenRenameClipboard(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeList:
			updatedM, cmd := nativeScreen.UpdateScreenNativeList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenNativeActions:
			updatedM, cmd := nativeScreen.UpdateScreenNativeActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandsList:
			updatedM, cmd := projectCmdScreen.UpdateScreenProjectCommandsList(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		case app.ScreenProjectCommandActions:
			updatedM, cmd := projectCmdScreen.UpdateScreenProjectCommandActions(pm.M, typedMsg, pm.ProjectRegistry)
			pm.M = updatedM
			return pm, cmd
		default:
			// Forward non-key/custom messages to current screen when needed (e.g., ticks)
			switch pm.M.CurrentScreen {
			case app.ScreenMain:
				updatedM, cmd := mainScreen.UpdateScreenMain(pm.M, typedMsg, pm.ProjectRegistry)
				pm.M = updatedM
				return pm, cmd
			default:
				return pm, nil
			}
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
		return sharedScreens.ViewSelectScreen(pm.M)
	case app.ScreenMain:
		return mainScreen.ViewMainScreen(pm.M, pm.ProjectRegistry)
	case app.ScreenLogin:
		return loginScreen.ViewScreenLogin(pm.M)
	case app.ScreenFilenamePrompt:
		return promptScreen.ViewFilenamePrompt(pm.M, pm.ProjectRegistry)
	case app.ScreenInstallDetails:
		return mainScreen.ViewInstallDetailsScreen(pm.M)
	case app.ScreenSettings:
		return settingsScreen.ViewSettingsScreen(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandHistory:
		return historyScreen.ViewScreenCommandHistory(pm.M, pm.ProjectRegistry)
	case app.ScreenCommandsCategory:
		return categoryScreen.ViewScreenCommandsCategory(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardList:
		return clipboardScreen.ViewScreenClipboardList(pm.M, pm.ProjectRegistry)
	case app.ScreenClipboardActions:
		return clipboardScreen.ViewScreenClipboardActions(pm.M, pm.ProjectRegistry)
	case app.ScreenRenameClipboard:
		return clipboardScreen.ViewScreenRenameClipboard(pm.M)
	case app.ScreenNativeList:
		return nativeScreen.ViewScreenNativeList(pm.M, pm.ProjectRegistry)
	case app.ScreenNativeActions:
		return nativeScreen.ViewScreenNativeActions(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandsList:
		return projectCmdScreen.ViewScreenProjectCommandsList(pm.M, pm.ProjectRegistry)
	case app.ScreenProjectCommandActions:
		if cli.IsDebugEnabled() {
			fmt.Fprintf(os.Stderr, "DEBUG: Routing to ViewScreenProjectCommandActions\n")
			fmt.Fprintf(os.Stderr, "DEBUG:   pm.M.SelectedProjectCommand = '%s'\n", pm.M.SelectedProjectCommand)
			registryIsNil := pm.ProjectRegistry == nil
			fmt.Fprintf(os.Stderr, "DEBUG:   pm.ProjectRegistry == nil: %t\n", registryIsNil)
		}
		return projectCmdScreen.ViewScreenProjectCommandActions(pm.M, pm.ProjectRegistry)
	case app.ScreenChoicePrompt:
		return promptScreen.ViewChoicePrompt(pm.M, pm.ProjectRegistry)
	}
	return ""
}

func main() {
	// Enable debug early if --debug present in raw args
	raw := os.Args[1:]
	for _, a := range raw {
		if a == "--debug" {
			cli.SetDebugEnabled(true)
			break
		}
	}
	// Enable verbose early if --verbose present
	for _, a := range raw {
		if a == "--verbose" {
			cli.SetVerboseEnabled(true)
			break
		}
	}
	if cli.IsDebugEnabled() {
		fmt.Println("DEBUG: main() function started.")
	}
	args := os.Args[1:] // Get arguments excluding program name

	// --- Load Project Registry ---
	if cli.IsDebugEnabled() {
		fmt.Println("DEBUG: Attempting to load project registry...")
	}
	projectRegistry, err := project.LoadProjectRegistry()
	if err != nil {
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Error loading project registry: %v\n", err)
		}
		fmt.Printf("Warning: Could not load project registry: %v\n", err)
		// Continue with an empty registry rather than failing
		projectRegistry = &project.ProjectRegistry{
			Projects:     make(map[string]project.ProjectInfo),
			LastUsedPath: "",
			GlobalUsages: 0,
		}
	} else {
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Project registry loaded successfully from %s. Contains %d projects. Global usages: %d\n",
				projectRegistry.RegistryPath, len(projectRegistry.Projects), projectRegistry.GlobalUsages)
		}
	}

	// --- Update .nextgen/nextgen-cli-commands.mdc on every run ---
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if writeErr := args_pkg.WriteNextgenCommandsMDC(cwd, projectRegistry); writeErr != nil {
			fmt.Printf("Warning: Failed to update .nextgen/nextgen-cli-commands.mdc: %v\n", writeErr)
		}
	} else {
		fmt.Printf("Warning: Could not determine working directory to update commands MDC: %v\n", cwdErr)
	}

	// Create the registry checker bridge
	registryChecker := commandRegistryCheckerBridge{}

	// --- Direct Command Execution Handling ---
	if len(args) > 0 {
		// Pass the registry checker to the parser
		parsedArgs := cli.ParseCommandLineArgs(args, registryChecker)
		if parsedArgs.BoolFlags["debug"] {
			cli.SetDebugEnabled(true)
		}
		if parsedArgs.BoolFlags["verbose"] {
			cli.SetVerboseEnabled(true)
		}

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
				if cli.IsDebugEnabled() {
					fmt.Printf("DEBUG: Command name '%s' recognized, proceeding to executeAndExit...\n", parsedArgs.CommandName)
				}
				executeAndExit(parsedArgs, projectRegistry)
			}
		} else {
			if cli.IsDebugEnabled() {
				fmt.Println("DEBUG: Command name NOT recognized by parser.")
			}
			if isHelpIntent {
				// General help requested
				displayGeneralHelp()
				os.Exit(0)
			} else {
				// No command, no help, no version - invalid usage
				fmt.Println("Error: Invalid arguments or flags provided without a command name.")
				_, variants := cliNameVariants()
				if len(variants) > 0 {
					fmt.Printf("Run `%s --help` for usage.\n", variants[0])
				} else {
					fmt.Println("Run `ng --help` for usage.")
				}
				os.Exit(1)
			}
		}
	}

	// --- Interactive Mode Fallback ---
	// If no args were provided (or version/help/command handled above), start the TUI
	fmt.Println("No command-line arguments provided, starting interactive mode...")

	// Get current directory for project detection
	if cli.IsDebugEnabled() {
		fmt.Println("DEBUG: Attempting to get current working directory...")
	}
	currentDir, err := os.Getwd()
	if err != nil {
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Error getting working directory: %v\n", err)
		}
		fmt.Printf("Warning: Could not determine current directory: %v\n", err)
		currentDir = "" // Default to empty if unable to determine
	} else {
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Current working directory: %s\n", currentDir)
		}
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
	clipboardPaginator.PerPage = 7
	clipboardPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("•")
	clipboardPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	nativePaginator := paginator.New()
	nativePaginator.Type = paginator.Dots
	nativePaginator.PerPage = 7
	nativePaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	nativePaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	projectCommandsPaginator := paginator.New()
	projectCommandsPaginator.Type = paginator.Dots
	projectCommandsPaginator.PerPage = 7
	projectCommandsPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	projectCommandsPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	mainListPaginator := paginator.New()
	mainListPaginator.Type = paginator.Dots
	mainListPaginator.PerPage = 5
	mainListPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	mainListPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	// NEW: Initialize History Paginator
	historyPaginator := paginator.New()
	historyPaginator.Type = paginator.Dots
	historyPaginator.PerPage = 7
	historyPaginator.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff3600")).Render("•")
	historyPaginator.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("•")

	// Build your initial model
	initialModel := app.Model{
		IsLoggedIn:               false,
		CurrentScreen:            app.ScreenLogin,
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
		HistoryPaginator:         historyPaginator, // Add history paginator
	}

	// Load persisted auth state from config
	if cfg, err := config.LoadConfig(); err == nil {
		initialModel.IsLoggedIn = cfg.IsLoggedIn
		if initialModel.IsLoggedIn {
			initialModel.CurrentScreen = app.ScreenMain
		} else {
			initialModel.CurrentScreen = app.ScreenLogin
		}
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
    // After TUI exits, print any prepared exit log
    if strings.TrimSpace(exitLog) != "" {
        fmt.Println(exitLog)
    }
}

// displayGeneralHelp prints the top-level help message.
func displayGeneralHelp() {
	fmt.Println("NextGen Go CLI - Help")
	fmt.Println("Usage:")
	_, variants := cliNameVariants()
	for _, n := range variants {
		fmt.Printf("  %s [command] [variables...] [--flags...]\n", n)
	}
	fmt.Println("Run without arguments to enter interactive mode.")

	allCmds := args_pkg.GetAllCommands()
	if len(allCmds) > 0 {
		fmt.Println("\nAvailable Commands:")
		sort.Slice(allCmds, func(i, j int) bool {
			return allCmds[i].Name() < allCmds[j].Name()
		})
		for _, cmd := range allCmds {
			fmt.Printf("  %-15s %s\n", cmd.Name(), cmd.Description())
		}
		if len(variants) > 0 {
			fmt.Printf("\nRun '%s [command] --help' for more information on a specific command.\n", variants[0])
		}
	} else {
		fmt.Println("\nNo commands registered yet.")
	}
	fmt.Println("\nGlobal Flags: --help, -h, --version")
}

// displayCommandHelp displays detailed help for a specific command.
func displayCommandHelp(commandName string) {
	cmd, found := args_pkg.GetCommand(commandName)
	if !found {
		fmt.Printf("Error: Unknown command '%s'\n", commandName)
		displayGeneralHelp() // Show general help as fallback
		return
	}

	// Display detailed help for the command
	fmt.Println("Usage:")
	usageSig := strings.TrimSpace(fmt.Sprintf("%s %s", cmd.Name(), cmd.Usage()))
	fmt.Println(formatUsageBoth("", usageSig))
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
	if cli.IsDebugEnabled() {
		fmt.Println("DEBUG: Attempting to get current working directory for execution...")
	}
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Warning: Could not determine current directory for command execution: %v\n", err)
	}
	if cli.IsDebugEnabled() {
		fmt.Printf("DEBUG: Current working directory for execution: %s\n", currentDir)
	}

	if cli.IsVerboseEnabled() {
		fmt.Printf("Attempting direct execution for command: %s\n", parsedArgs.CommandName)
		fmt.Printf("Variables: %v\n", parsedArgs.Variables)
		fmt.Printf("Flags: %v\n", parsedArgs.Flags)
		fmt.Printf("BoolFlags: %v\n", parsedArgs.BoolFlags)
	}

	// If this is an args-based command, validate required args/flags first
	if cmd, found := args_pkg.GetCommand(parsedArgs.CommandName); found {
		if err := template_cmds.ValidateArgs(parsedArgs, cmd.ExpectedArgs(), cmd.ExpectedFlags()); err != nil {
			// Show a helpful guide for correct usage
			fmt.Printf("Error: %v\n\n", err)
			displayCommandHelp(parsedArgs.CommandName)
			os.Exit(1)
		}
	}

	err = executeDirectCommand(parsedArgs, registry) // Pass registry
	if err != nil {
		fmt.Printf("Error executing command '%s': %v\n", parsedArgs.CommandName, err)
		os.Exit(1)
	}
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
	if cmd, found := args_pkg.GetCommand(commandName); found {
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Executing command '%s' via args package...\n", commandName)
		}
		execErr = cmd.Execute(args)
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Args command '%s' finished. Error: %v\n", commandName, execErr)
		}
		// Placeholders are not directly available from args commands for history

	} else if registry != nil && registry.NativeCommands != nil && registry.NativeCommands[commandName] != "" {
		// 2. Try executing as a User-Saved Native Command
		commandString := registry.NativeCommands[commandName]
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Executing command '%s' as user-saved native command...\n", commandName)
		}
		fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
		execErr = runShellCommand(commandString, commandArgs, projectPath) // nolint:SA4006 -- value is used after branching

	} else if registry != nil && registry.ClipboardCommands != nil && registry.ClipboardCommands[commandName].Template != "" {
		// 3. Try executing as a Clipboard Command
		clipboardSpec := registry.ClipboardCommands[commandName]
		if cli.IsDebugEnabled() {
			fmt.Printf("DEBUG: Executing command '%s' as clipboard command...\n", commandName)
		}
		templateBytes := []byte(clipboardSpec.Template)
		keys := template_cmds.InferVariableKeys(string(templateBytes))
		if len(keys) != len(commandArgs) {
			usageParts := make([]string, len(keys))
			for i, k := range keys {
				usageParts[i] = fmt.Sprintf("<%s>", k)
			}
			usage := formatUsageBoth(commandName, strings.Join(usageParts, " "))
			return fmt.Errorf("clipboard command '%s' requires %d argument(s): %s\nUsage: %s",
				commandName, len(keys), strings.Join(keys, ", "), usage)
		} else {
			varsMap := make(map[string]string)
			for i, key := range keys {
				varsMap[key] = commandArgs[i]
			}
			placeholders = template_cmds.BuildPlaceholders(varsMap) // Store placeholders
			if cli.IsDebugEnabled() {
				fmt.Printf("DEBUG: Running clipboard template with placeholders: %+v\n", placeholders)
			}
			template_cmds.CreatedFiles = []string{}
			template_cmds.EditedIndexers = make(map[string]bool)
			execErr = template_cmds.ExecuteJSONTemplateFromMemory(templateBytes, projectPath, placeholders) // nolint:SA4006 -- value is used after branching
		}

	} else {
		// 4. Try executing as a Project Command; if not found, fall back to Built-in Template Command
		executedProject := false
		if projectPath != "." {
			localCmdDir := filepath.Join(projectPath, ".nextgen", "local-commands")
			kebabName := template_cmds.ToKebabCase(commandName)
			cmdFilePath := filepath.Join(localCmdDir, kebabName+".json")
			if _, err := os.Stat(cmdFilePath); err == nil {
				jsonData, readErr := os.ReadFile(cmdFilePath)
				if readErr != nil {
					execErr = fmt.Errorf("failed to read project command file '%s': %w", cmdFilePath, readErr)
				} else {
					// First try to parse as a shell command file
					var cmdData projectCommandFile
					if json.Unmarshal(jsonData, &cmdData) == nil && strings.TrimSpace(cmdData.Command) != "" {
						commandString := cmdData.Command
						if cli.IsDebugEnabled() {
							fmt.Printf("DEBUG: Executing command '%s' as project command...\n", commandName)
						}
						fmt.Printf("  Command: %s\n  Args: %v\n", commandString, commandArgs)
						execErr = runShellCommand(commandString, commandArgs, projectPath)
						// Placeholders not applicable
						executedProject = true
					} else {
						// Otherwise, try to treat it as a template JSON
						var generic map[string]interface{}
						if json.Unmarshal(jsonData, &generic) == nil {
							if fp, ok := generic["filePaths"]; ok {
								if arr, okArr := fp.([]interface{}); okArr && len(arr) > 0 {
									if cli.IsDebugEnabled() {
										fmt.Printf("DEBUG: Executing command '%s' as project template command...\n", commandName)
									}
									keys := template_cmds.InferVariableKeys(string(jsonData))
									if len(keys) != len(commandArgs) {
										usageParts := make([]string, len(keys))
										for i, k := range keys {
											usageParts[i] = fmt.Sprintf("<%s>", k)
										}
										usage := formatUsageBoth(commandName, strings.Join(usageParts, " "))
										return fmt.Errorf(
											"command '%s' requires %d argument(s): %s\nUsage: %s",
											commandName, len(keys), strings.Join(keys, ", "), usage,
										)
									} else {
										varsMap := make(map[string]string)
										for i, key := range keys {
											varsMap[key] = commandArgs[i]
										}
										placeholders = template_cmds.BuildPlaceholders(varsMap)
										template_cmds.CreatedFiles = []string{}
										template_cmds.EditedIndexers = make(map[string]bool)
										execErr = template_cmds.ExecuteJSONTemplateFromMemory(jsonData, projectPath, placeholders)
									}
									executedProject = true
								} else {
									execErr = fmt.Errorf("invalid project command file '%s': missing 'command' or 'filePaths'", cmdFilePath)
								}
							} else {
								execErr = fmt.Errorf("invalid project command file '%s': missing 'filePaths'", cmdFilePath)
							}
						} else {
							execErr = fmt.Errorf("failed to parse project command file '%s'", cmdFilePath)
						}
					}
				}
			}

			// 5. If not executed as a project command, try executing as a Built-in Template Command
			if !executedProject {
				spec := template_cmds.GetCommandSpec(commandName)
				if spec.TemplatePath != "" {
					if cli.IsDebugEnabled() {
						fmt.Printf("DEBUG: Executing command '%s' as built-in template command...\n", commandName)
					}
					templateBytes, loadErr := template_cmds.LoadCommandTemplate(spec.TemplatePath)
					if loadErr != nil {
						execErr = fmt.Errorf("failed to load template %s: %w", spec.TemplatePath, loadErr)
					} else {
						keys := template_cmds.InferVariableKeys(string(templateBytes))
						if len(keys) != len(commandArgs) {
							usageParts := make([]string, len(keys))
							for i, k := range keys {
								usageParts[i] = fmt.Sprintf("<%s>", k)
							}
							usage := formatUsageBoth(commandName, strings.Join(usageParts, " "))
							return fmt.Errorf("command '%s' requires %d argument(s): %s\nUsage: %s",
								commandName, len(keys), strings.Join(keys, ", "), usage)
						} else {
							varsMap := make(map[string]string)
							for i, key := range keys {
								varsMap[key] = commandArgs[i]
							}
							placeholders = template_cmds.BuildPlaceholders(varsMap) // Store placeholders
							if cli.IsDebugEnabled() {
								fmt.Printf("DEBUG: Running template with placeholders: %+v\n", placeholders)
							}
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
		}

		// --- Record History (Centralized Logic) ---
		if execErr == nil { // Only record history if execution was successful
			historicCmd := project.HistoricCommand{
				Name:           commandName,
				Variables:      placeholders, // Will be nil for non-template commands, which is fine
				Timestamp:      time.Now().Unix(),
				GeneratedFiles: append([]string{}, template_cmds.CreatedFiles...), // Copy generated files
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

	return nil
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
