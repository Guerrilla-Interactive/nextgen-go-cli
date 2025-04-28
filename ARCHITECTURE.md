# NextGen Go CLI Architecture

## Project Overview

NextGen Go CLI is a terminal-based user interface application built with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework. It provides an interactive command-line interface for developers to execute various commands with a beautiful TUI (Terminal User Interface).

The application detects project frameworks (like React, Next.js, Tailwind CSS, etc.) and offers a command palette tailored to the project context. It uses an MVC-like architecture where screens represent different views, and commands are executed based on user input.

## Core Architecture

### Directory Structure

```
nextgen-go-cli/
├── app/
│   ├── commands/                    # Command execution logic
│   │   ├── command-helpers.go       # Helper functions for command execution
│   │   ├── command-registry.go      # Command definitions and registry
│   │   └── *.json                   # JSON template files (embedded)
│   ├── screens/                     # UI screens for different parts of the application
│   │   ├── all-commands.screen.go   # "All Commands" screen implementation
│   │   ├── filename-prompt.screen.go # Input prompts for filenames/variables
│   │   ├── install-details.screen.go # Command execution results screen
│   │   ├── intro.screen.go          # Introductory/welcome screen
│   │   ├── recent-commands.screen.go # Main screen with recent commands
│   │   ├── screen-helpers.go        # Shared UI helper functions
│   │   └── screen-init.go           # Initialization functions for screens
│   ├── utils/                      # Utility functions
│   │   └── filetree.go             # File tree visualization utilities
│   ├── app.go                      # Core application model definitions
│   └── projectstats.go             # Project statistics detection and formatting
└── main.go                         # Application entry point
```

### Key Files and Their Purposes

#### Entry Point
- **`main.go`** - Application bootstrap, initializes the Bubble Tea program with the initial model state, sets up the core program loop, and handles global error conditions.

#### Application Core
- **`app/app.go`** - Defines the central `Model` struct that holds all application state, screen type enum, and core styling constants used throughout the app.
- **`app/projectstats.go`** - Contains functions for detecting, grouping, and displaying project framework information.

#### Screen Implementations
- **`app/screens/recent-commands.screen.go`** - Implements the main screen showing recent commands and action row.
- **`app/screens/all-commands.screen.go`** - Implements the screen that displays all available commands in a grid.
- **`app/screens/filename-prompt.screen.go`** - Handles the input prompts for collecting variables from the user.
- **`app/screens/install-details.screen.go`** - Shows the results of command execution with a file tree.
- **`app/screens/intro.screen.go`** - Implements the welcome/introduction screen (though often skipped).
- **`app/screens/screen-helpers.go`** - Contains shared helper functions used across multiple screens.
- **`app/screens/screen-init.go`** - Provides initialization functions for project detection and screen setup.

#### Command System
- **`app/commands/command-registry.go`** - Defines available commands, manages the command registry, and implements command execution.
- **`app/commands/command-helpers.go`** - Contains helper functions for template processing, placeholder substitution, and file operations.

#### Utilities
- **`app/utils/filetree.go`** - Implements file tree visualization using Unicode box-drawing characters and icons.

### Dependencies (go.mod)
The application relies on several key packages:
- `github.com/charmbracelet/bubbletea` - Core TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling library
- `github.com/charmbracelet/bubbles` - UI components for Bubble Tea
- `github.com/atotto/clipboard` - Clipboard interaction

## Detailed Concepts and Their Implementations

### 1. The Bubble Tea TEA Pattern

The application uses The Elm Architecture (TEA) pattern through the Bubble Tea framework, which is implemented across multiple files:

- **Implementation Location**: `main.go` (core loop) and all screen files
- **Key Components**:
  - **Init** - Defined in `main.go:ProgramModel.Init()`
  - **Update** - Implemented in `main.go:ProgramModel.Update()` and screen-specific functions like `screens/recent-commands.screen.go:UpdateScreenMain()`
  - **View** - Implemented in `main.go:ProgramModel.View()` and screen-specific functions like `screens/recent-commands.screen.go:ViewMainScreen()`

**Example from `main.go`**:
```go
func (pm ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch typedMsg := msg.(type) {
    case app.Model:
        pm.M = typedMsg
        return pm, nil
    case tea.KeyMsg:
        switch pm.M.CurrentScreen {
        case app.ScreenMain:
            updatedM, cmd := screens.UpdateScreenMain(pm.M, typedMsg)
            pm.M = updatedM
            return pm, cmd
        // ... other screen handlers
        }
    }
    return pm, nil
}
```

### 2. Screen Navigation System

The screen navigation system controls transitions between different views:

- **Definition Location**: `app/app.go:Screen` (enum type)
- **Implementation Location**: 
  - All screen update functions (e.g., `screens/recent-commands.screen.go:UpdateScreenMain()`)
  - Transition logic in `screens/screen-helpers.go:HandleCommandSelection()`

**Example from `screen-helpers.go`**:
```go
func HandleCommandSelection(m *app.Model, itemName string) *app.Model {
    recordCommand(m, itemName)

    if strings.ToLower(itemName) == "view project stats" {
        m.CurrentScreen = app.ScreenProjectStats
        return m
    }
    
    // ... more transition logic
    
    m.CurrentScreen = app.ScreenFilenamePrompt
    return m
}
```

### 3. Model Definition and State Management

The central Model structure defines all application state:

- **Definition Location**: `app/app.go:Model` struct
- **Usage Locations**: 
  - `main.go` (initial state)
  - All screen files (state updates)
  - `screens/screen-helpers.go` (helper functions that modify state)

**Example from `app.go`**:
```go
type Model struct {
    CurrentScreen Screen
    IsLoggedIn    bool
    SelectedIndex int
    AllCmdsIndex  int
    // ... many more fields
}
```

### 4. Command Registry and Execution

The command system defines available commands and handles their execution:

- **Definition Location**: `app/commands/command-registry.go`
- **Template Storage**: JSON files embedded in the binary via Go 1.16+ embed directive
- **Execution Flow**: 
  1. Command selection in UI (`screens/recent-commands.screen.go` or `screens/all-commands.screen.go`)
  2. Parameter collection (`screens/filename-prompt.screen.go`)
  3. Command execution (`app/commands/command-registry.go:RunCommand()`)
  4. Result display (`screens/install-details.screen.go`)

**Example from `command-registry.go`**:
```go
//go:embed *.json
var commandFiles embed.FS

// Commands is our single authoritative list of all possible commands.
var Commands = []CommandSpec{
    {Name: "add section"}, // no template (placeholder)
    {Name: "remove section"},
    {Name: "add page", TemplatePath: "page-and-archive.json"},
    // ... more commands
}

func RunCommand(cmdName, projectPath string, placeholders map[string]string) error {
    // ... command execution logic
}
```

### 5. Variable Collection System

For commands that require input, the variable collection system handles user input:

- **Definition Location**: Multi-variable support in `app/app.go:Model` (several fields)
- **Implementation Location**: `screens/filename-prompt.screen.go`
- **Variable Detection**: `screens/screen-helpers.go:requiresMultipleVars()` and `extractVariableKeys()`

**Example from `filename-prompt.screen.go`**:
```go
func UpdateScreenFilenamePrompt(m app.Model, keyMsg tea.KeyMsg) (app.Model, tea.Cmd) {
    // ... handle input

    if m.MultipleVariables {
        // Multi-variable mode
        currentKey := m.VariableKeys[m.CurrentVariableIndex]
        m.Variables[currentKey] = value
        m.CurrentVariableIndex++
        
        // ... check if all variables collected
    } else {
        // Single variable mode
        // ... process single input
    }
    
    return m, nil
}
```

### 6. UI Layout and Styling System

The UI is built using Lipgloss styles and complex layout composition:

- **Style Definitions**: `app/app.go` (global styles)
- **Layout Components**: 
  - `screens/screen-helpers.go:baseContainer()` and `sideContainer()`
  - Screen-specific layout in view functions
- **Panel System**: Used in screens to create multi-column layouts

**Example from `recent-commands.screen.go`**:
```go
func ViewMainScreen(m app.Model) string {
    // ... build content

    leftPanel := baseContainer(body)
    rightPanel := sideContainer(preview)

    fixedLeftPanel := lipgloss.Place(
        lipgloss.Width(leftPanel),
        termHeight,
        lipgloss.Left,
        lipgloss.Bottom,
        leftPanel,
    )

    return lipgloss.JoinHorizontal(lipgloss.Bottom, fixedLeftPanel, rightPanel)
}
```

### 7. File Tree Visualization

The file tree visualization system renders directory trees:

- **Definition Location**: `app/utils/filetree.go`
- **Tree Node Structure**: `app/utils/filetree.go:FileNode`
- **Usage Location**: `screens/install-details.screen.go` (to show created files)

**Example from `filetree.go`**:
```go
func RenderFileTree(node *FileNode, prefix string, isLast bool, skipSelf bool, isEdited IsEditedFunc) string {
    // ... tree rendering logic with Unicode branch characters
    branch := routeStyle.Render("┣")
    if isLast {
        branch = routeStyle.Render("┗")
    }
    // ... more rendering logic with recursion
}
```

### 8. Project Framework Detection

The project detection system identifies frameworks used in the project:

- **Definition Location**: `app/projectstats.go`
- **Implementation**: 
  - Detection in `screens/screen-init.go:detectFrameworks()`
  - Grouping in `app/projectstats.go:GroupRecognizedPackages()`
  - Display in `app/projectstats.go:RenderPackagesHorizontally()`

**Example from `screen-init.go`**:
```go
func detectFrameworks(projectPath string) []string {
    knownPackages := map[string]string{
        "next":              "Next.js",
        "sanity":            "Sanity (CMS)",
        "tailwindcss":       "Tailwind CSS",
        // ... more package mappings
    }
    
    // ... analyze package.json and return detected frameworks
}
```

### 9. Command Preview System

The command preview system shows a live preview of command results:

- **Implementation Location**: `screens/filename-prompt.screen.go` (live preview during typing)
- **Preview Generation**: `app/commands/command-helpers.go:GeneratePreviewFileTree()`

**Example from `filename-prompt.screen.go`**:
```go
// In single variable mode, update live preview
{
    input := m.TempFilename
    if strings.TrimSpace(input) == "" {
        input = "Filename"
    }
    
    // ... build placeholders
    
    if preview, err := commands.GeneratePreviewFileTree(m.PendingCommand, placeholderMap, m.ProjectPath); err == nil {
        m.LivePreview = preview
    } else {
        m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
    }
}
```

### 10. JSON Template System

The application uses JSON templates to define what files should be created:

- **Template Storage**: Embedded JSON files in `app/commands/*.json`
- **Template Structure**: Defined in `app/commands/command-helpers.go:JSONCommandTemplate`
- **Template Processing**: `app/commands/command-helpers.go:ExecuteJSONTemplate()`

**Example from `command-helpers.go`**:
```go
type JSONCommandTemplate struct {
    FilePaths []FilePathGroup `json:"filePaths"`
}

type FilePathGroup struct {
    Key   string     `json:"_key"`
    Type  string     `json:"_type"`
    ID    string     `json:"id"`
    Nodes []TreeNode `json:"nodes"`
    Path  string     `json:"path"`
}

func ExecuteJSONTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
    // ... process JSON template and create/modify files
}
```

## File Interdependencies Map

Here's how the key files depend on each other:

1. **Entry Point Chain**:
   - `main.go` → initializes → `app.Model` (from `app/app.go`)
   - `main.go` → calls → screen functions (from `app/screens/*.go`)

2. **Screen Update Chain**:
   - `main.go:ProgramModel.Update()` → calls → `screens/*:UpdateScreen*()` → updates → `app.Model`
   - `screens/*:UpdateScreen*()` → calls → `screens/screen-helpers.go` functions (for shared functionality)

3. **Command Execution Chain**:
   - `screens/*:UpdateScreen*()` → calls → `commands/command-registry.go:RunCommand()`
   - `commands/command-registry.go:RunCommand()` → calls → `commands/command-helpers.go:RunJsonTemplateBytes()`
   - After execution, sends → `screens.CommandFinishedMsg` → back to → `main.go:ProgramModel.Update()`

4. **View Rendering Chain**:
   - `main.go:ProgramModel.View()` → calls → `screens/*:ViewScreen*()` → uses → `app.Model`
   - `screens/*:ViewScreen*()` → calls → `screens/screen-helpers.go` functions (for UI components)
   - `screens/install-details.screen.go` → uses → `app/utils/filetree.go` (for file tree rendering)

5. **Project Detection Chain**:
   - `screens/screen-init.go:InitProjectCmd()` → calls → `screens/screen-init.go:detectFrameworks()`
   - `screens/*:ViewScreen*()` → uses → `app/projectstats.go` (for displaying detected frameworks)

## Coding Style and Component Integration

### Function Naming Conventions

The codebase follows consistent function naming conventions:

1. **Screen-specific functions**: Prefixed with screen name
   - `UpdateScreenMain` - Handles input for the main screen
   - `ViewMainScreen` - Renders the main screen
   - `UpdateScreenFilenamePrompt` - Handles input for the filename prompt screen

2. **Navigation helpers**: Named with action verbs
   - `moveSelectionUp` - Moves selection up in a list
   - `moveSelectionDown` - Moves selection down in a list
   - `moveAllCmdsSelectionLeft` - Moves selection left in the all commands screen

3. **Rendering helpers**: Prefixed with "render"
   - `renderFileTree` - Renders a file tree structure
   - `renderPackagesHorizontally` - Renders packages in a horizontal layout
   - `renderItemList` - Renders a generic item list

### Screen Component Structure

Each screen in the application follows a consistent structure:

1. **Update function**: Processes user input and updates the model
   ```go
   func UpdateScreenMain(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
       // Handle key presses
       switch msg.String() {
       case "ctrl+c", "q":
           os.Exit(0)
       case "up", "k":
           m = moveSelectionUp(m)
       // ...
       }
       return m, nil
   }
   ```

2. **View function**: Renders the screen based on current model state
   ```go
   func ViewMainScreen(m app.Model) string {
       // Build UI components
       title := app.TitleStyle.Render("Title")
       body := renderComponents(m)
       // ...
       return baseContainer(body)
   }
   ```

3. **Helper functions**: Screen-specific utility functions
   ```go
   func getItemName(m app.Model, index int) (string, bool) {
       // Helper logic
   }
   ```

### Immutable State Pattern

The application maintains immutability by creating new model instances rather than modifying the existing one:

```go
// Instead of modifying m directly
func moveSelectionUp(m app.Model) app.Model {
    if m.SelectedIndex > 0 {
        m.SelectedIndex--  // Create a copy of m with updated index
    } else {
        m.SelectedIndex = total - 1
    }
    return m  // Return the updated model
}
```

### UI Composition Techniques

The UI is composed using a hierarchical approach with Lipgloss styles:

1. **Style definitions**: Global styles defined in `app.go`
   ```go
   var TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
   var HighlightStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFA500"))
   ```

2. **Container patterns**: Consistent containers for UI elements
   ```go
   func baseContainer(content string) string {
       containerStyle := lipgloss.NewStyle().
           Padding(1, 2).
           Margin(1)
       return containerStyle.Render(content)
   }
   ```

3. **Composite layouts**: Building complex layouts from simpler components
   ```go
   // Join vertical and horizontal elements
   body := lipgloss.JoinVertical(lipgloss.Left,
       lipgloss.JoinHorizontal(lipgloss.Bottom, leftPanel, rightPanel),
       app.HelpStyle.Render("(Use arrow keys or j/k/h/l to move; q quits.)"),
   )
   ```

4. **Panel anchoring**: Anchoring panels to specific parts of the terminal
   ```go
   // Anchor the panel to the bottom
   anchoredPanel := lipgloss.Place(
       lipgloss.Width(panel),
       termHeight,
       lipgloss.Left,
       lipgloss.Bottom,
       panel,
   )
   ```

### Message Passing System

The application uses a sophisticated message passing system following the TEA pattern:

1. **Custom message types**: Define specific messages for events
   ```go
   type CommandFinishedMsg struct {
       Err error
   }
   ```

2. **Asynchronous commands**: Functions that return commands producing messages
   ```go
   return m, func() tea.Msg {
       err := commands.RunCommand(cmdName, m.ProjectPath, nil)
       return CommandFinishedMsg{Err: err}
   }
   ```

3. **Message handling**: Processing messages in Update functions
   ```go
   case CommandFinishedMsg:
       if typedMsg.Err != nil {
           // Handle error
       }
       // Update state
       pm.M.CurrentScreen = app.ScreenInstallDetails
   ```

### Navigation System Implementation

The application uses a sophisticated navigation system:

1. **Index-based selection**: Items are selected using indices
   ```go
   if m.SelectedIndex == fullIndex {
       // Highlight the selected item
       line += colStyle.Render(app.HighlightStyle.Render(iconCmd))
   } else {
       line += colStyle.Render(app.ChoiceStyle.Render(iconCmd))
   }
   ```

2. **Column-major navigation**: Navigation works in column-major order
   ```go
   func moveAllCmdsSelectionDown(m app.Model) app.Model {
       idx := m.AllCmdsIndex
       // ... complex logic for column-major navigation
       const rows = 10
       col := idx / rows
       row := idx % rows
       // ... update indices
       return m
   }
   ```

3. **Smart wrapping**: Selection wraps around edges intelligently
   ```go
   if row == 0 {
       // Move to "Back" when at top row and going up
       m.AllCmdsIndex = commandsCount
       return m
   }
   ```

### Screen Transition Flow

Screen transitions follow a predictable pattern:

1. **Update current screen**: Set the next screen
   ```go
   m.CurrentScreen = app.ScreenFilenamePrompt
   ```

2. **Prepare screen-specific state**: Set up state for the target screen
   ```go
   m.PendingCommand = itemName
   m.TempFilename = ""
   m.LivePreview = ""
   ```

3. **Return updated model**: Pass the updated model to Bubble Tea
   ```go
   return m, nil
   ```

### File Tree Rendering System

The file tree is rendered using a recursive approach:

1. **Tree construction**: Build a tree structure from file paths
   ```go
   func BuildFileTree(paths []string) *FileNode {
       root := &FileNode{Name: "", Children: make(map[string]*FileNode)}
       for _, fullPath := range paths {
           // ... build tree nodes
       }
       return root
   }
   ```

2. **Recursive rendering**: Render the tree using recursion
   ```go
   func RenderFileTree(node *FileNode, prefix string, isLast bool, skipSelf bool, isEdited IsEditedFunc) string {
       // ... render current node
       for i, name := range names {
           child := node.Children[name]
           childIsLast := i == len(names)-1
           result += RenderFileTree(child, newPrefix, childIsLast, false, isEdited)
       }
       return result
   }
   ```

3. **Branch character styling**: Style tree branches with colors
   ```go
   branch := routeStyle.Render("┣")
   if isLast {
       branch = routeStyle.Render("┗")
   }
   ```

### Command Preview System

The command preview system provides real-time feedback:

1. **Placeholder building**: Create placeholders for command variables
   ```go
   placeholders := commands.BuildPlaceholders(map[string]string{keys[0]: "Filename"})
   ```

2. **Preview generation**: Generate a preview based on the command and placeholders
   ```go
   if preview, err := commands.GeneratePreviewFileTree(cmdName, placeholders, m.ProjectPath); err == nil {
       m.LivePreview = preview
   }
   ```

3. **Live updates**: Update the preview as the user types
   ```go
   input := m.TempFilename
   if strings.TrimSpace(input) == "" {
       input = "Filename"  // Default placeholder
   }
   // ... generate preview with current input
   ```

### Error Handling Approach

Error handling follows consistent patterns:

1. **Error messages in UI**: Display errors in the UI
   ```go
   m.LivePreview = fmt.Sprintf("Preview unavailable: %v", err)
   ```

2. **Error propagation**: Propagate errors through message system
   ```go
   case CommandFinishedMsg:
       if typedMsg.Err != nil {
           // Handle error
           fmt.Println("Command finished with error:", typedMsg.Err)
       }
   ```

3. **Graceful fallbacks**: Provide fallbacks when errors occur
   ```go
   if preview, err := commands.GeneratePreviewFileTree(...); err == nil {
       m.LivePreview = preview
   } else {
       m.LivePreview = "Preview unavailable."  // Fallback
   }
   ```

## Component Integration Patterns

### Screen-Command Integration

Screens integrate with commands through a defined pattern:

1. **Command selection**: User selects a command in the UI
   ```go
   itemName, _ := getItemName(m, m.SelectedIndex)
   ```

2. **Command recording**: Record command usage
   ```go
   recordCommand(&m, itemName)
   ```

3. **Command execution**: Execute the command asynchronously
   ```go
   return m, func() tea.Msg {
       err := commands.RunCommand(itemName, m.ProjectPath, placeholders)
       return CommandFinishedMsg{Err: err}
   }
   ```

4. **Command result handling**: Process command results
   ```go
   case CommandFinishedMsg:
       // Show install details
       m.CurrentScreen = app.ScreenInstallDetails
   ```

### UI Layout Composition

UI layouts are composed through a layered approach:

1. **Content building**: Build the content string
   ```go
   title := app.TitleStyle.Render("Title")
   body := title + "\n" + content
   ```

2. **Container wrapping**: Wrap the content in a container
   ```go
   leftPanel := baseContainer(body)
   ```

3. **Multi-panel layout**: Combine multiple panels
   ```go
   return lipgloss.JoinHorizontal(lipgloss.Bottom, leftPanel, rightPanel)
   ```

4. **Terminal anchoring**: Anchor the layout to terminal dimensions
   ```go
   return lipgloss.Place(
       lipgloss.Width(panel),
       termHeight,
       lipgloss.Left,
       lipgloss.Bottom,
       panel,
   )
   ```

### Model-View Separation

The application maintains clear separation between model and view:

1. **Model updates**: Update the model in Update functions
   ```go
   func UpdateScreenMain(m app.Model, msg tea.KeyMsg) (app.Model, tea.Cmd) {
       // Update model based on input
       return m, cmd
   }
   ```

2. **View rendering**: Render the view based on current model
   ```go
   func ViewMainScreen(m app.Model) string {
       // Render UI based on model
       return ui
   }
   ```

3. **State propagation**: Pass model state through function parameters
   ```go
   func renderRecentUsedInColumns(items []string, m *app.Model, offset, columns, rows int) string {
       // Use model for rendering
   }
   ```

### Helper Function Integration

Helper functions are integrated through a consistent pattern:

1. **Pure functions**: Functions that don't modify state
   ```go
   func ToPascalCase(input string) string {
       // Pure string transformation
   }
   ```

2. **Model transformers**: Functions that transform the model
   ```go
   func moveSelectionUp(m app.Model) app.Model {
       // Transform model and return new instance
   }
   ```

3. **UI component generators**: Functions that generate UI components
   ```go
   func renderActionRowItems(items []string, m *app.Model, offset, columns int) string {
       // Generate UI string
   }
   ```

This architecture enables a clean, maintainable codebase with clear separation of concerns, consistent patterns, and predictable behavior across the application. 