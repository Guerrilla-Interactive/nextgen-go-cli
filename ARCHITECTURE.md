# NextGen Go CLI Architecture

## Project Overview

NextGen Go CLI is a terminal-based user interface application built with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework. It provides an interactive command-line interface for developers to execute various template-based commands with a TUI (Terminal User Interface).

The application detects project frameworks and offers a command palette tailored to the project context. It uses an MVC-like architecture where screens represent different views, and commands are executed based on user input.

## Core Architecture

### Directory Structure (Simplified)

```
nextgen-go-cli/
├── app/
│   ├── app.go                      # Core application model, screen enum, messages
│   ├── cli/                        # CLI argument parsing logic
│   ├── commands/                   # Command template definition, execution, helpers
│   │   ├── args/                   # Arg-based CLI command implementations
│   │   ├── command-helpers.go
│   │   ├── command-registry.go
│   │   └── native-commands/        # Embedded JSON template files
│   ├── project/                    # Project detection & persistent registry
│   │   ├── project-detector.go
│   │   └── project-tracker.go
│   ├── screens/                    # UI screens (Feature-based subdirs planned)
│   │   ├── main/                   # (Planned)
│   │   ├── settings/               # Settings screen logic
│   │   │   └── settings.screen.go
│   │   └── ... (other screen files)
│   │   └── shared/                 # (Planned)
│   │       ├── screen-helpers.go
│   │       └── screen-init.go
│   └── utils/                      # Utility functions (e.g., file tree)
└── main.go                         # Application entry point (CLI & TUI)
```

### Key Files and Their Purposes

*   **`main.go`**: Application bootstrap, handles CLI argument parsing and direct execution, or starts the TUI (Bubble Tea program). Contains the main `Update` and `View` logic loop.
*   **`app/app.go`**: Defines the central `app.Model` struct (application state), the `Screen` enum for navigation, custom message types (`app.CommandFinishedMsg`), and global Lipgloss styles.
*   **`app/commands/command-registry.go`**: Defines built-in command specifications (`CommandSpec`), loads embedded native command templates (`*.json`), provides helpers like `GetCommandSpec`.
*   **`app/commands/command-helpers.go`**: Contains core logic for executing JSON templates (`ExecuteJSONTemplateFromMemory`), placeholder substitution (`BuildPlaceholders`, etc.), snippet merging (`smartMerge`), file tree preview generation (`GeneratePreviewFileTree`), and the TUI command runner (`RunCommand`).
*   **`app/commands/args/`**: Contains implementations for commands executed directly via the CLI using flags and arguments.
*   **`app/project/project-detector.go`**: Logic to detect project type and technologies based on files like `package.json`.
*   **`app/project/project-tracker.go`**: Manages the persistent `ProjectRegistry` (saved in `~/.config/nextgen-cli/projects.json`), tracks project usage, command history, clipboard commands, native commands, and favorites.
*   **`app/screens/`**: Contains individual Go files for each UI screen or feature area. Each typically has an `Update*` function (handling input/state changes) and a `View*` function (rendering the UI).
    *   **`settings/settings.screen.go`**: Example of a feature-specific screen package.
    *   **`screen-helpers.go`**: Shared helper functions used across multiple screens (e.g., `HandleCommandSelection`).
    *   **`screen-init.go`**: Initialization logic, like detecting the initial project.
*   **`app/utils/`**: General utility functions, like file tree rendering.
*   **`app/cli/`**: Logic specifically for parsing command-line arguments.

## Detailed Concepts and Their Implementations

### 1. The Bubble Tea TEA Pattern

*   **Implementation**: `main.go` (root `ProgramModel` wrapping `app.Model`), screen files (`Update*`, `View*` functions).
*   **Model**: `app.Model` defined in `app/app.go` holds all shared state.
*   **Update**: `main.go:ProgramModel.Update` is the central message handler. It delegates to screen-specific `Update*` functions based on `m.CurrentScreen`.
*   **View**: `main.go:ProgramModel.View` calls the appropriate screen-specific `View*` function.
*   **Messages**: Custom message types like `app.CommandFinishedMsg` (defined in `app/app.go`) are used for communication, especially for async operations.

### 2. Screen Navigation System

*   **Definition**: `Screen` enum in `app/app.go`.
*   **Implementation**: Screen transitions are managed by setting `m.CurrentScreen` in the various `Update*` functions (e.g., in `app/screens/screen-helpers.go:HandleCommandSelection`, or in screen-specific handlers for actions like "Back").

### 3. Command Execution (TUI vs CLI)

*   **TUI Execution Flow**:
    1.  User selects command in a screen UI.
    2.  `HandleCommandSelection` (or similar screen logic) determines if variables are needed.
    3.  If needed, transitions to `ScreenFilenamePrompt`.
    4.  Once variables are collected (or if none were needed), the screen's `Update*` function calls `commands.RunCommand` (`command-helpers.go`).
    5.  `RunCommand` determines the command source (clipboard, project, built-in), reads the appropriate template content, and returns an async `tea.Cmd`.
    6.  The async function executes the template using `ExecuteJSONTemplateFromMemory`.
    7.  Upon completion, it returns an `app.CommandFinishedMsg` containing results (error, files generated, etc.).
    8.  `main.go:ProgramModel.Update` receives the message.
    9.  If the command succeeded (`msg.Err == nil`), it calls `registry.RecordCommandHistory` (`project-tracker.go`).
    10. Transitions to `ScreenInstallDetails`.
*   **CLI Execution Flow**:
    1.  `main.go` parses args using `app/cli/` logic.
    2.  If a command is recognized, `executeDirectCommand` (`main.go`) is called.
    3.  `executeDirectCommand` checks command type (args-based, native shell, clipboard, project, built-in template).
    4.  It executes the command directly (using `cmd.Execute`, `runShellCommand`, or `ExecuteJSONTemplateFromMemory`).
    5.  If successful, it calls `registry.RecordCommandHistory` directly.
    6.  Exits the application.

### 4. Persistent State (Project Registry)

*   **Implementation**: `app/project/project-tracker.go` (`ProjectRegistry` struct).
*   **Storage**: JSON file (`~/.config/nextgen-cli/projects.json`).
*   **Data Stored**: Known project paths, usage counts, last access times, command history per project, saved clipboard commands, saved native shell commands, favorites.
*   **Loading**: `LoadProjectRegistry` called in `main.go`.
*   **Saving**: `Save` method called explicitly after modifications (e.g., in `AddOrUpdateProject`, `RecordCommandHistory`, and when toggling favorites or managing clipboard/native commands).

### 5. Command History

*   **Structure**: `project.HistoricCommand` struct defined in `project/project-info.go` (or similar).
*   **Storage**: Stored within the `ProjectInfo` struct for each project in the `ProjectRegistry`.
*   **Recording**: Centralized in `project.ProjectRegistry.RecordCommandHistory`. Called from `main.go` for both TUI (`CommandFinishedMsg`) and CLI (`executeDirectCommand`) successful executions.
*   **Viewing**: `app/screens/command-history.screen.go` displays the history for the current project.

### 6. JSON Template System & Snippet Merging

*   **Templates**: Defined in `.json` files (e.g., in `app/commands/native-commands/`).
*   **Structure**: `JSONCommandTemplate`, `FilePathGroup`, `TreeNode` structs in `app/commands/command-helpers.go`.
*   **Execution**: `ExecuteJSONTemplateFromMemory` processes the template structure.
*   **File Handling**: `gatherNodes` handles directory creation and file writing/merging.
*   **Snippet Merging**: `smartMerge` function looks for `// ADD SNIPPET_KEY ABOVE/BELOW` markers in existing files and inserts corresponding `// START OF SNIPPET_KEY ... // END OF SNIPPET_KEY` blocks from the template code.

### 7. File Tree Preview & Rendering

*   **Preview Generation**: `GeneratePreviewFileTree`, `GeneratePreviewFileTreeFromClipboard`, etc., in `app/commands/command-helpers.go` parse templates *without* writing files to determine the resulting structure.
*   **Rendering**: `app/utils/filetree.go` takes a list of relative paths, builds a tree structure (`FileNode`), and renders it using `RenderFileTree`.

## File Interdependencies Map (Updated)

1.  **`main.go`** -> `app` (Model, Msgs), `project` (Registry), `cli`, `screens/*` (Update/View delegates), `commands` (for CLI execution), `utils`.
2.  **`app/screens/*`** -> `app` (Model, Screens, Styles), `commands` (RunCommand, GetKeys), `project` (Registry access), `utils`, other `screens` (for shared helpers or navigation).
3.  **`app/commands/command-helpers.go`** -> `app` (Msg), `project` (Registry for RunCommand), `cli`, `utils`, `clipboard`.
4.  **`app/project/project-tracker.go`** -> `app` (dependency on `HistoricCommand` if defined there, otherwise self-contained or depends on its own types).
5.  **`app/app.go`** -> Defines core types, minimal external dependencies (Lipgloss, Bubbles).

(Note: Careful management is needed to avoid import cycles, especially between `screens` and `commands`.)

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

## Navigation Example: Viewing Settings (Updated)

1.  **User selects "View Settings"** from the main command list (`ScreenMain`).
2.  `screens.UpdateScreenMain` handles the `Enter` key press.
3.  It identifies the selected item.
4.  It updates the model: `m.CurrentScreen = app.ScreenSettings`.
5.  The main `Update` loop in `main.go` receives the updated model.
6.  The main `View` loop calls `settings.ViewSettingsScreen` (via `main.go:ProgramModel.View` switch).
7.  `settings.ViewSettingsScreen` renders the settings categories and details.
8.  User navigates within the Settings screen using `settings.UpdateScreenSettings`.
9.  User selects "Back" or presses `Esc`.
10. `settings.UpdateScreenSettings` sets `m.CurrentScreen = app.ScreenMain`.
11. Control returns to the main screen.

## TUI Command Execution Example (Updated)

1.  User selects "add page" on `ScreenMain`.
2.  `screens.HandleCommandSelection` determines variables are needed.
3.  Transitions to `ScreenFilenamePrompt` (`m.CurrentScreen = app.ScreenFilenamePrompt`).
4.  User enters "MyNewPage" and presses Enter.
5.  `screens.UpdateScreenFilenamePrompt` builds placeholders (`{ "Filename": "MyNewPage", ... }`) and calls `commands.RunCommand("add page", path, placeholders, registry)`.
6.  `commands.RunCommand` finds the built-in template for "add page", prepares an async function (`tea.Cmd`).
7.  `main.go` receives the `tea.Cmd` and executes it.
8.  The async function runs `ExecuteJSONTemplateFromMemory`, creating files.
9.  It returns `app.CommandFinishedMsg{ Err: nil, CommandName: "add page", ..., GeneratedFiles: [...] }`.
10. `main.go:ProgramModel.Update` receives the `CommandFinishedMsg`.
11. Since `Err` is nil, it calls `registry.RecordCommandHistory` with the details from the message.
12. Sets `m.CurrentScreen = app.ScreenInstallDetails`.
13. `main.go:ProgramModel.View` calls `screens.ViewInstallDetailsScreen` to show results.

This updated architecture document should better reflect the current state of the project. 