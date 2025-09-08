package commands

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/Guerrilla-Interactive/nextgen-go-cli/app"
    "github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
    "github.com/Guerrilla-Interactive/nextgen-go-cli/app/project"
    "github.com/atotto/clipboard"
    tea "github.com/charmbracelet/bubbletea"
)

// -----------------------------------------------------------------------------
// [RUNTIME] Visibility, state, command execution & loaders
// -----------------------------------------------------------------------------

// CommandVisibility defines optional conditions for when a command should be shown.
type CommandVisibility struct {
    PackageJSON              map[string]string         `json:"packageJson"`
    PackageJSONArrayContains map[string]string         `json:"packageJsonArrayContains"`
    AnyOf                    []CommandVisibilityClause `json:"anyOf"`
    CommandPackagesContains  []string                  `json:"commandPackagesContains"`
}

// CommandVisibilityClause represents a single OR clause.
type CommandVisibilityClause struct {
    PackageJSON              map[string]string `json:"packageJson"`
    PackageJSONArrayContains map[string]string `json:"packageJsonArrayContains"`
    CommandPackagesContains  []string          `json:"commandPackagesContains"`
}

func isPackageJSONMatch(projectPath string, expected map[string]string) bool {
    if len(expected) == 0 { return true }
    pkgPath := filepath.Join(projectPath, "package.json")
    b, err := os.ReadFile(pkgPath)
    if err != nil { return false }
    var data map[string]any
    if err := json.Unmarshal(b, &data); err != nil { return false }
    for k, v := range expected {
        if actual, ok := data[k]; ok {
            switch t := actual.(type) {
            case string:
                if strings.TrimSpace(t) != strings.TrimSpace(v) { return false }
            default:
                if fmt.Sprint(t) != v { return false }
            }
        } else { return false }
    }
    return true
}

func isPackageJSONArrayContains(projectPath string, expected map[string]string) bool {
    if len(expected) == 0 { return true }
    pkgPath := filepath.Join(projectPath, "package.json")
    b, err := os.ReadFile(pkgPath)
    if err != nil { return false }
    var data map[string]any
    if err := json.Unmarshal(b, &data); err != nil { return false }
    for key, want := range expected {
        raw, ok := data[key]
        if !ok { return false }
        arr, ok := raw.([]any)
        if !ok { return false }
        found := false
        for _, v := range arr {
            if s, ok := v.(string); ok && strings.TrimSpace(s) == strings.TrimSpace(want) { found = true; break }
        }
        if !found { return false }
    }
    return true
}

// isCommandPackagesContains returns true if .nextgen/command-packages.json contains all expected tokens.
func isCommandPackagesContains(projectPath string, expected []string) bool {
    if len(expected) == 0 { return true }
    p := filepath.Join(projectPath, ".nextgen", "command-packages.json")
    b, err := os.ReadFile(p)
    if err != nil { return false }
    trim := strings.TrimSpace(string(b))
    if trim == "" { return false }
    var arr []string
    if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
        set := make(map[string]bool, len(arr))
        for _, s := range arr { set[strings.TrimSpace(s)] = true }
        for _, want := range expected { if !set[strings.TrimSpace(want)] { return false } }
        return true
    }
    var obj map[string]any
    if err := json.Unmarshal(b, &obj); err == nil {
        collected := map[string]bool{}
        if raw, ok := obj["identifiers"]; ok {
            if a, ok2 := raw.([]any); ok2 { for _, v := range a { if s, ok3 := v.(string); ok3 { collected[strings.TrimSpace(s)] = true } } }
        }
        if len(collected) == 0 {
            for _, v := range obj {
                if a, ok2 := v.([]any); ok2 {
                    for _, vv := range a { if s, ok3 := vv.(string); ok3 { collected[strings.TrimSpace(s)] = true } }
                }
            }
        }
        if len(collected) > 0 {
            for _, want := range expected { if !collected[strings.TrimSpace(want)] { return false } }
            return true
        }
    }
    return false
}

func matchesVisibilityClause(projectPath string, clause CommandVisibilityClause) bool {
    if len(clause.PackageJSON) > 0 && !isPackageJSONMatch(projectPath, clause.PackageJSON) { return false }
    if len(clause.PackageJSONArrayContains) > 0 && !isPackageJSONArrayContains(projectPath, clause.PackageJSONArrayContains) { return false }
    if len(clause.CommandPackagesContains) > 0 && !isCommandPackagesContains(projectPath, clause.CommandPackagesContains) { return false }
    return true
}

// IsCommandVisible evaluates whether a command should be shown for the given project path.
func IsCommandVisible(spec CommandSpec, projectPath string) bool {
    if spec.Visibility == nil { return true }
    if len(spec.Visibility.AnyOf) > 0 {
        for _, c := range spec.Visibility.AnyOf { if matchesVisibilityClause(projectPath, c) { return true } }
        return false
    }
    if len(spec.Visibility.PackageJSON) > 0 { if !isPackageJSONMatch(projectPath, spec.Visibility.PackageJSON) { return false } }
    if len(spec.Visibility.PackageJSONArrayContains) > 0 { if !isPackageJSONArrayContains(projectPath, spec.Visibility.PackageJSONArrayContains) { return false } }
    if len(spec.Visibility.CommandPackagesContains) > 0 { if !isCommandPackagesContains(projectPath, spec.Visibility.CommandPackagesContains) { return false } }
    return true
}

// -----------------------------
// Runtime state
// -----------------------------

// Global variable to record created file paths.
var CreatedFiles []string

// EditedIndexers holds file paths that are indexers and have been edited.
var EditedIndexers = make(map[string]bool)

// MarkEditedIndexer marks the given file path as an edited indexer.
func MarkEditedIndexer(path string) { EditedIndexers[path] = true }

// RecordCreatedFile appends a created file path to the global CreatedFiles list.
func RecordCreatedFile(path string) {
    for _, p := range CreatedFiles { if p == path { return } }
    CreatedFiles = append(CreatedFiles, path)
}

// -----------------------------
// Command execution & loaders
// -----------------------------

// RunCommand executes a command template (clipboard / local / built-in) asynchronously for the TUI.
func RunCommand(cmdName, projectPath string, placeholders map[string]string, registry *project.ProjectRegistry) tea.Cmd {
    CreatedFiles = []string{}
    EditedIndexers = make(map[string]bool)

    localPlaceholders := make(map[string]string)
    if placeholders != nil { for k, v := range placeholders { localPlaceholders[k] = v } }

    return func() tea.Msg {
        var err error
        var executionSource string
        var templateBytes []byte

        if strings.ToLower(cmdName) == "paste from clipboard" {
            clipboardContent, readErr := clipboard.ReadAll()
            if readErr != nil { err = fmt.Errorf("failed to read clipboard for paste command: %w", readErr) } else {
                templateBytes = []byte(clipboardContent)
                executionSource = "clipboard content"
            }
        } else if strings.HasSuffix(strings.ToLower(cmdName), ".json") {
            if embeddedBytes, readErr := LoadCommandTemplate(cmdName); readErr == nil {
                templateBytes = embeddedBytes
                executionSource = "embedded path"
            } else { err = fmt.Errorf("template path %s not found: %w", cmdName, readErr) }
        } else {
            if registry != nil && registry.ClipboardCommands != nil {
                if clipSpec, found := registry.ClipboardCommands[cmdName]; found {
                    templateBytes = []byte(clipSpec.Template)
                    executionSource = fmt.Sprintf("clipboard command '%s'", cmdName)
                }
            }
            if templateBytes == nil && projectPath != "" && projectPath != "." {
                localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands")
                kebabName := ToKebabCase(cmdName)
                cmdFilePath := filepath.Join(localCmdPath, kebabName+".json")
                if _, statErr := os.Stat(cmdFilePath); statErr == nil {
                    fileBytes, readErr := os.ReadFile(cmdFilePath)
                    if readErr == nil {
                        templateBytes = fileBytes
                        executionSource = fmt.Sprintf("project command '%s'", kebabName+".json")
                    } else { err = fmt.Errorf("error reading project command file %s: %w", cmdFilePath, readErr) }
                } else if !os.IsNotExist(statErr) { err = fmt.Errorf("error checking project command file %s: %w", cmdFilePath, statErr) }
            }
            if templateBytes == nil && err == nil {
                spec := GetCommandSpec(cmdName)
                if spec.TemplatePath != "" {
                    embeddedBytes, readErr := LoadCommandTemplate(spec.TemplatePath)
                    if readErr == nil {
                        templateBytes = embeddedBytes
                        executionSource = fmt.Sprintf("built-in template %s", spec.TemplatePath)
                    } else { err = fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr) }
                }
            }
        }

        if templateBytes != nil && err == nil {
            err = ExecuteJSONTemplateFromMemory(templateBytes, projectPath, localPlaceholders)
            if err != nil { err = fmt.Errorf("error executing template for command '%s' from %s: %w", cmdName, executionSource, err) }
        } else if err == nil {
            err = fmt.Errorf("command '%s' not found or has no associated template for TUI execution", cmdName)
        }

        return app.CommandFinishedMsg{
            Err:            err,
            CommandName:    cmdName,
            ProjectPath:    projectPath,
            Placeholders:   localPlaceholders,
            GeneratedFiles: append([]string{}, CreatedFiles...),
        }
    }
}

// UpsertClipboardCommand overwrites or adds a clipboard command by name and saves the registry.
func UpsertClipboardCommand(registry *project.ProjectRegistry, name string, template string) error {
    if registry == nil { return fmt.Errorf("registry unavailable") }
    if registry.ClipboardCommands == nil { registry.ClipboardCommands = make(map[string]project.ClipboardCommandSpec) }
    registry.ClipboardCommands[name] = project.ClipboardCommandSpec{
        Name:       name,
        Template:   template,
        IsFavorite: registry.ClipboardCommands[name].IsFavorite,
        Timestamp:  time.Now().Unix(),
    }
    return registry.Save()
}

func LoadTemplateBytesForName(cmdName, projectPath string, registry *project.ProjectRegistry) ([]byte, string, error) {
    if registry != nil && registry.ClipboardCommands != nil {
        if clipSpec, found := registry.ClipboardCommands[cmdName]; found { return []byte(clipSpec.Template), "clipboard", nil }
    }
    if projectPath != "" && projectPath != "." {
        localCmdPath := filepath.Join(projectPath, ".nextgen", "local-commands")
        kebabName := ToKebabCase(cmdName)
        cmdFilePath := filepath.Join(localCmdPath, kebabName+".json")
        if _, statErr := os.Stat(cmdFilePath); statErr == nil {
            fileBytes, readErr := os.ReadFile(cmdFilePath)
            if readErr == nil { return fileBytes, "project", nil }
            return nil, "", fmt.Errorf("error reading project command file %s: %w", cmdFilePath, readErr)
        }
    }
    spec := GetCommandSpec(cmdName)
    if spec.TemplatePath != "" {
        embeddedBytes, readErr := LoadCommandTemplate(spec.TemplatePath)
        if readErr == nil { return embeddedBytes, "builtin", nil }
        return nil, "", fmt.Errorf("error reading embedded template %s: %w", spec.TemplatePath, readErr)
    }
    return nil, "", fmt.Errorf("template not found for %s", cmdName)
}

// IsCompositeTemplate returns true if the template JSON defines run steps without filePaths.
func IsCompositeTemplate(templateBytes []byte) bool {
    var t struct { FilePaths []any `json:"filePaths"`; Run []RunStep `json:"run"` }
    if err := json.Unmarshal(templateBytes, &t); err != nil { return false }
    return len(t.FilePaths) == 0 && len(t.Run) > 0
}

// GetCompositeRunSlugs returns the list of slugs referenced by run steps.
func GetCompositeRunSlugs(templateBytes []byte) ([]string, error) {
    var t struct { Run []RunStep `json:"run"` }
    if err := json.Unmarshal(templateBytes, &t); err != nil { return nil, err }
    var slugs []string
    for _, s := range t.Run { if strings.ToLower(s.Type) == "invoke" && strings.TrimSpace(s.Slug) != "" { slugs = append(slugs, s.Slug) } }
    return slugs, nil
}

// ResolveCommandTitleBySlug returns a friendly name for a command identified by slug or name.
func ResolveCommandTitleBySlug(nameOrSlug string) string {
    spec := GetCommandSpec(nameOrSlug)
    if spec.Name != "" { return spec.Name }
    parts := strings.Split(ToKebabCase(nameOrSlug), "-")
    for i := range parts { if len(parts[i]) > 0 { parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:] } }
    return strings.Join(parts, " ")
}

// -----------------------------
// CLI argument validation
// -----------------------------

// ValidateArgs checks the provided CommandArgs against the command's definitions.
func ValidateArgs(parsedArgs cli.CommandArgs, expectedArgs []cli.ArgDef, expectedFlags []cli.FlagDef) error {
    requiredArgCount := 0
    for _, argDef := range expectedArgs { if argDef.Required { requiredArgCount++ } }
    allowsTrailingArgs := false
    if len(expectedArgs) > 0 && strings.HasSuffix(expectedArgs[len(expectedArgs)-1].Name, "...") { allowsTrailingArgs = true }

    if len(parsedArgs.Variables) < requiredArgCount {
        var requiredNames []string
        for i := 0; i < requiredArgCount; i++ { requiredNames = append(requiredNames, fmt.Sprintf("<%s>", expectedArgs[i].Name)) }
        return fmt.Errorf("missing required arguments: %s", strings.Join(requiredNames, " "))
    }

    if !allowsTrailingArgs && len(parsedArgs.Variables) > len(expectedArgs) {
        return fmt.Errorf("too many arguments provided. Expected max %d, got %d", len(expectedArgs), len(parsedArgs.Variables))
    }

    for _, flagDef := range expectedFlags {
        if flagDef.Required {
            _, longExists := parsedArgs.Flags[flagDef.Name]
            _, shortExists := parsedArgs.Flags[flagDef.ShortName]
            _, longBoolExists := parsedArgs.BoolFlags[flagDef.Name]
            _, shortBoolExists := parsedArgs.BoolFlags[flagDef.ShortName]

            found := longExists || (flagDef.ShortName != "" && shortExists) || longBoolExists || (flagDef.ShortName != "" && shortBoolExists)
            if !found {
                flagName := "--" + flagDef.Name
                if flagDef.ShortName != "" { flagName += "/-" + flagDef.ShortName }
                return fmt.Errorf("missing required flag: %s", flagName)
            }
        }
    }
    return nil
}

