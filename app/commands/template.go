package commands

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"

    "github.com/Guerrilla-Interactive/nextgen-go-cli/app/cli"
    "github.com/Guerrilla-Interactive/nextgen-go-cli/app/utils"
)

// -----------------------------------------------------------------------------
// [TEMPLATE] Structs & file creation/merge pipeline
// -----------------------------------------------------------------------------

// JSONCommandTemplate is the root structure of your template JSON file.
type JSONCommandTemplate struct {
    FilePaths      []FilePathGroup `json:"filePaths"`
    Args           []ArgDef        `json:"args"`
    Run            []RunStep       `json:"run"`
    AutoBrowseRoot string          `json:"autoBrowseRoot"`
}

// FilePathGroup describes a target path in your project plus an array of TreeNode objects.
type FilePathGroup struct {
    Key   string     `json:"_key"`
    Type  string     `json:"_type"`
    ID    string     `json:"id"`
    Nodes []TreeNode `json:"nodes"`
    Path  string     `json:"path"`
}

// TreeNode describes either a directory (with children) or a file (with code).
type TreeNode struct {
    Key       string            `json:"_key"`
    Type      string            `json:"_type"`
    Children  []TreeNode        `json:"children"`
    Code      string            `json:"code"`
    ID        string            `json:"id"`
    Name      string            `json:"name"`
    IsIndexer bool              `json:"isIndexer"` // even if false, we'll override if we see the marker in the code
    Markers   []InsertionMarker `json:"markers"`
}

// InsertionMarker describes a desired insertion marker and a fallback anchor.
type InsertionMarker struct {
    Mark     string         `json:"mark"`
    Fallback MarkerFallback `json:"fallback"`
}

// MarkerFallback supports legacy string fallbacks and structured object fallbacks.
type MarkerFallback struct {
    Raw  string              // legacy fallback body
    Spec *MarkerFallbackSpec // structured fallback spec
}

type MarkerFallbackSpec struct {
    Target        string `json:"target"`
    // Optional start/end anchors for block replacement
    TargetStart   string `json:"targetStart"`
    TargetEnd     string `json:"targetEnd"`
    Behaviour     string `json:"behaviour"` // insertAfter | insertBefore | insertBeforeInline | insertAfterInline | insertAfterLine | insertBeforeLine | insertNextLine
    Content       string `json:"content"`
    FallbackOnly  bool   `json:"fallbackOnly"`
    Occurrence    string `json:"occurrence"` // first | last
    RequireAbsent string `json:"requireAbsent"`
    Replacement   string `json:"replacement"`
}

// UnmarshalJSON allows MarkerFallback to be a string or an object.
func (m *MarkerFallback) UnmarshalJSON(b []byte) error {
    s := strings.TrimSpace(string(b))
    if len(s) == 0 || s == "null" {
        return nil
    }
    if s[0] == '"' {
        var v string
        if err := json.Unmarshal(b, &v); err != nil { return err }
        m.Raw = v
        m.Spec = nil
        return nil
    }
    if s[0] == '{' {
        var spec MarkerFallbackSpec
        if err := json.Unmarshal(b, &spec); err != nil { return err }
        m.Spec = &spec
        m.Raw = ""
        return nil
    }
    return nil
}

// ArgDef describes a variable to ask the user for.
type ArgDef struct {
    Name         string   `json:"name"`
    Type         string   `json:"type"` // text, select
    Message      string   `json:"message"`
    Choices      []Choice `json:"choices"`
    Default      string   `json:"default"`
    Required     bool     `json:"required"`
    RequiredWhen *struct {
        Var    string `json:"var"`
        Equals string `json:"equals"`
    } `json:"requiredWhen"`
}

type Choice struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

// RunStep allows invoking a subcommand based on a condition.
type RunStep struct {
    Type        string   `json:"type"` // invoke
    Slug        string   `json:"slug"`
    When        string   `json:"when"`
    ForwardVars []string `json:"forwardVars"`
}

// ExecuteJSONTemplate reads your JSON command file and creates the specified files/folders.
func ExecuteJSONTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
    templateBytes, err := os.ReadFile(jsonFilePath)
    if err != nil { return fmt.Errorf("could not read JSON template: %w", err) }
    var template JSONCommandTemplate
    if err := json.Unmarshal(templateBytes, &template); err != nil { return fmt.Errorf("could not parse JSON template: %w", err) }
    for _, group := range template.FilePaths {
        basePath := filepath.Join(projectPath, group.Path)
        if err := gatherNodes(group.Nodes, basePath, projectPath, placeholders); err != nil {
            return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
        }
    }
    return nil
}

// gatherNodes creates directories or files; merges indexers using smartMerge and markers.
func gatherNodes(nodes []TreeNode, basePath, projectPath string, placeholders map[string]string) error {
    for _, node := range nodes {
        nodeName := replacePlaceholders(node.Name, placeholders)
        currentPath := filepath.Join(basePath, nodeName)

        if len(node.Children) > 0 {
            if err := os.MkdirAll(currentPath, 0755); err != nil { return fmt.Errorf("failed to create directory %s: %w", currentPath, err) }
            if err := gatherNodes(node.Children, currentPath, projectPath, placeholders); err != nil { return err }
            continue
        }

        if node.Code == "" { continue }

        if err := os.MkdirAll(filepath.Dir(currentPath), 0755); err != nil { return fmt.Errorf("failed to create parent directory for %s: %w", currentPath, err) }
        code := replacePlaceholders(node.Code, placeholders)

        // Detect indexer
        isIndexer := node.IsIndexer
        if !isIndexer {
            indexerMarker := regexp.MustCompile(`(?m)^\s*//\s*THIS\s+IS\s+AN\s+INDEXER\s+FILE`)
            if indexerMarker.FindStringIndex(code) != nil || strings.Contains(code, "// THIS IS AN INDEXER FILE") {
                isIndexer = true
                if cli.IsVerboseEnabled() { fmt.Printf("ℹ️  Detected indexer marker in file %s, registering as an indexer file.\n", currentPath) }
            }
        }
        if !isIndexer {
            if snippetMap, _ := extractSnippets(code); len(snippetMap) > 0 {
                isIndexer = true
                if cli.IsVerboseEnabled() { fmt.Printf("ℹ️  Treating %s as indexer based on presence of snippet groups.\n", currentPath) }
            }
        }

        if _, err := os.Stat(currentPath); err == nil {
            // Existing file
            if isIndexer {
                existingContentBytes, readErr := os.ReadFile(currentPath)
                if readErr != nil { return fmt.Errorf("failed to read existing file %s: %w", currentPath, readErr) }
                existingContent := string(existingContentBytes)

                // Ensure explicit node markers exist; insert via fallback if missing.
                if len(node.Markers) > 0 {
                    tmplSnippets, _ := extractSnippets(code)
                    for _, m := range node.Markers {
                        mk := strings.TrimSpace(m.Mark)
                        if mk == "" { continue }
                        if m.Fallback.Spec != nil {
                            beh := strings.ToLower(strings.TrimSpace(m.Fallback.Spec.Behaviour))
                            if beh == "replaceifmissing" || beh == "replaceifabsent" || beh == "replace" {
                                tgt := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                                rep := replacePlaceholders(m.Fallback.Spec.Replacement, placeholders)
                                req := replacePlaceholders(m.Fallback.Spec.RequireAbsent, placeholders)
                                occ := m.Fallback.Spec.Occurrence
                                if modified, did := conditionalReplace(existingContent, tgt, req, rep, occ); did {
                                    existingContent = modified
                                    if cli.IsVerboseEnabled() { fmt.Printf("✓ Replaced inline for '%s' in %s.\n", mk, currentPath) }
                                }
                                continue
                            }
                            // Replace a block between start/end anchors
                            if beh == "replacebetween" || beh == "replaceblock" || beh == "replacerange" {
                                start := replacePlaceholders(m.Fallback.Spec.TargetStart, placeholders)
                                if strings.TrimSpace(start) == "" { start = replacePlaceholders(m.Fallback.Spec.Target, placeholders) }
                                end := replacePlaceholders(m.Fallback.Spec.TargetEnd, placeholders)
                                rep := replacePlaceholders(m.Fallback.Spec.Replacement, placeholders)
                                req := replacePlaceholders(m.Fallback.Spec.RequireAbsent, placeholders)
                                occ := m.Fallback.Spec.Occurrence
                                if modified, did := replaceBetweenAnchors(existingContent, start, end, req, rep, occ); did {
                                    existingContent = modified
                                    if cli.IsVerboseEnabled() { fmt.Printf("✓ Replaced block for '%s' in %s.\n", mk, currentPath) }
                                }
                                continue
                            }
                        }
                        // Inline-injection (same-line)
                        if m.Fallback.Spec != nil && (strings.EqualFold(m.Fallback.Spec.Behaviour, "insertBeforeInline") || strings.EqualFold(m.Fallback.Spec.Behaviour, "insertAfterInline")) {
                            var snip string
                            if strings.TrimSpace(m.Fallback.Spec.Content) != "" {
                                snip = replacePlaceholders(m.Fallback.Spec.Content, placeholders)
                            } else {
                                var ok bool
                                snip, ok = findSnippetForKeyGlobal(tmplSnippets, mk)
                                if !ok || strings.TrimSpace(snip) == "" { continue }
                            }
                            target := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                            behaviour := m.Fallback.Spec.Behaviour
                            occurrence := m.Fallback.Spec.Occurrence
                            if behaviour == "" { behaviour = "insertBeforeInline" }
                            if modified, inserted := insertSnippetInlineRelativeToTarget(existingContent, snip, target, behaviour, occurrence); inserted {
                                existingContent = modified
                                if cli.IsVerboseEnabled() { fmt.Printf("✓ Injected inline snippet for '%s' in %s.\n", mk, currentPath) }
                            }
                            continue
                        }
                        // Line-injection (new line before/after target line)
                        if m.Fallback.Spec != nil && (strings.EqualFold(m.Fallback.Spec.Behaviour, "insertBeforeLine") || strings.EqualFold(m.Fallback.Spec.Behaviour, "insertAfterLine") || strings.EqualFold(m.Fallback.Spec.Behaviour, "insertNextLine") || m.Fallback.Spec.FallbackOnly) {
                            var snip string
                            if strings.TrimSpace(m.Fallback.Spec.Content) != "" {
                                snip = replacePlaceholders(m.Fallback.Spec.Content, placeholders)
                            } else {
                                var ok bool
                                snip, ok = findSnippetForKeyGlobal(tmplSnippets, mk)
                                if !ok || strings.TrimSpace(snip) == "" { continue }
                            }
                            target := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                            behaviour := m.Fallback.Spec.Behaviour
                            occurrence := m.Fallback.Spec.Occurrence
                            if behaviour == "" { behaviour = "insertAfterLine" }
                            if modified, inserted := insertSnippetOnNewLineRelativeToTarget(existingContent, snip, target, behaviour, occurrence); inserted {
                                existingContent = modified
                                if cli.IsVerboseEnabled() { fmt.Printf("✓ Injected snippet on new line for '%s' in %s.\n", mk, currentPath) }
                            }
                            continue
                        }
                        // Marker creation (line-based or fallback block)
                        if !markerForKeyExists(existingContent, mk) {
                            var modified string
                            var inserted bool
                            if m.Fallback.Raw != "" {
                                modified, inserted = insertAddMarkerAfterFallback(existingContent, mk, replacePlaceholders(m.Fallback.Raw, placeholders))
                            } else if m.Fallback.Spec != nil {
                                target := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                                behaviour := m.Fallback.Spec.Behaviour
                                occurrence := m.Fallback.Spec.Occurrence
                                modified, inserted = insertAddMarkerRelativeToTarget(existingContent, mk, target, behaviour, occurrence)
                            }
                            if inserted {
                                existingContent = modified
                                if cli.IsVerboseEnabled() { fmt.Printf("ℹ️  Inserted missing marker for '%s' in %s using fallback.\n", mk, currentPath) }
                            }
                        }
                    }
                }

                // Heuristic auto-insert of markers if indexer lacks any markers.
                if !hasAnySnippetMarkers(existingContent) {
                    if snippetMap, _ := extractSnippets(code); len(snippetMap) > 0 {
                        sanitize := func(s string) string {
                            s = strings.ToUpper(strings.TrimSpace(s))
                            var b strings.Builder
                            for i := 0; i < len(s); i++ {
                                c := s[i]
                                if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') { b.WriteByte(c) }
                            }
                            return b.String()
                        }
                        var skipMarks []string
                        for _, mm := range node.Markers {
                            if mm.Fallback.Spec != nil && (strings.Contains(strings.ToLower(mm.Fallback.Spec.Behaviour), "inline") || mm.Fallback.Spec.FallbackOnly) {
                                if mks := strings.TrimSpace(mm.Mark); mks != "" { skipMarks = append(skipMarks, sanitize(mks)) }
                            }
                        }
                        var keys []string
                        for k := range snippetMap {
                            ks := sanitize(k)
                            skip := false
                            for _, sm := range skipMarks {
                                if ks == sm || strings.Contains(ks, sm) || strings.Contains(sm, ks) { skip = true; break }
                            }
                            if !skip { keys = append(keys, k) }
                        }
                        if len(keys) > 0 {
                            if modified, inserted := autoInsertIndexerMarkers(existingContent, keys); inserted {
                                existingContent = modified
                                if cli.IsVerboseEnabled() { fmt.Printf("ℹ️  Inserted %d indexer markers into %s.\n", len(keys), currentPath) }
                            }
                        }
                    }
                }

                // Merge template snippets (augmented with missing fallback snippets)
                codeWithFallbackSnippets := augmentTemplateWithFallbackSnippets(code, node.Markers, placeholders)
                mergedContent, mergeErr := smartMerge(existingContent, codeWithFallbackSnippets)
                if mergeErr != nil { return fmt.Errorf("failed to merge file %s: %w", currentPath, mergeErr) }
                mergedContent = cleanupIndexerContent(mergedContent)
                mergedContent = ensureExportForLinkReference(mergedContent)
                if err := os.WriteFile(currentPath, []byte(mergedContent), 0644); err != nil { return fmt.Errorf("failed to write merged file %s: %w", currentPath, err) }
                if cli.IsVerboseEnabled() { fmt.Printf("✓ Merged updates into existing file %s.\n", currentPath) }
                if rel, err := filepath.Rel(projectPath, currentPath); err == nil { MarkEditedIndexer(rel) } else { MarkEditedIndexer(currentPath) }
            } else {
                // Non-indexer overwrite
                newContent := removeSnippetMarkers(code)
                if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil { return fmt.Errorf("failed to overwrite file %s: %w", currentPath, err) }
                if cli.IsVerboseEnabled() { fmt.Printf("✓ Replaced existing file %s.\n", currentPath) }
            }
            if rel, err := filepath.Rel(projectPath, currentPath); err == nil { RecordCreatedFile(rel) } else { RecordCreatedFile(currentPath) }
            continue
        }

        // New file
        if isIndexer {
            newContent := removeSnippetMarkers(code)
            // Apply inline fallback injections (e.g., insertBeforeInline) for brand new files
            newContent = applyInlineFallbacksForNewFile(newContent, node, placeholders)
            newContent = cleanupIndexerContent(newContent)
            newContent = ensureExportForLinkReference(newContent)
            if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil { return fmt.Errorf("failed to write new indexer file %s: %w", currentPath, err) }
            if cli.IsVerboseEnabled() { fmt.Printf("✓ Created new indexer file %s.\n", currentPath) }
        } else {
            newContent := removeSnippetMarkers(code)
            // Apply inline fallback injections (e.g., insertBeforeInline) for brand new files
            newContent = applyInlineFallbacksForNewFile(newContent, node, placeholders)
            newContent = ensureExportForLinkReference(newContent)
            if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil { return fmt.Errorf("failed to write file %s: %w", currentPath, err) }
        }
        if rel, err := filepath.Rel(projectPath, currentPath); err == nil { RecordCreatedFile(rel) } else { RecordCreatedFile(currentPath) }
    }
    return nil
}

// replacePlaceholders walks through the placeholders map and replaces all occurrences.
func replacePlaceholders(content string, placeholders map[string]string) string {
    for oldVal, newVal := range placeholders { content = strings.ReplaceAll(content, oldVal, newVal) }
    return content
}

// RunJsonTemplate loads and executes a command template from a JSON file.
func RunJsonTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
    if err := ExecuteJSONTemplate(jsonFilePath, projectPath, placeholders); err != nil { return fmt.Errorf("failed to run JSON template: %w", err) }
    return nil
}

// RunJsonTemplateBytes loads and executes a command template from byte data.
func RunJsonTemplateBytes(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
    if err := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders); err != nil { return fmt.Errorf("failed to run JSON template from memory: %w", err) }
    return nil
}

// ExecuteJSONTemplateFromMemory executes the template logic given the JSON bytes.
func ExecuteJSONTemplateFromMemory(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
    var template JSONCommandTemplate
    if err := json.Unmarshal(jsonBytes, &template); err != nil { return fmt.Errorf("could not parse JSON template: %w", err) }
    for _, group := range template.FilePaths {
        basePath := filepath.Join(projectPath, group.Path)
        if err := gatherNodes(group.Nodes, basePath, projectPath, placeholders); err != nil {
            return fmt.Errorf("error processing nodes for path %s: %w", group.Path, err)
        }
    }
    return nil
}

// -----------------------------
// Placeholder helpers & casing
// -----------------------------

// ToKebabCase converts a string to kebab-case.
func ToKebabCase(input string) string {
    words := splitIntoWords(input)
    return strings.ToLower(strings.Join(words, "-"))
}

// ToPascalCase converts input to PascalCase.
func ToPascalCase(input string) string {
    words := splitIntoWords(input)
    for i, w := range words { words[i] = strings.Title(strings.ToLower(w)) }
    return strings.Join(words, "")
}

// ToCamelCase converts input to camelCase.
func ToCamelCase(input string) string {
    pascal := ToPascalCase(input)
    if len(pascal) == 0 { return pascal }
    return strings.ToLower(pascal[:1]) + pascal[1:]
}

// ToLowercase converts input to all lowercase.
func ToLowercase(input string) string { return strings.ToLower(input) }

// splitIntoWords splits a string into words based on hyphens or spaces.
func splitIntoWords(s string) []string { s = strings.ReplaceAll(s, "-", " "); return strings.Fields(s) }

// BuildPlaceholders creates a map of placeholder variables from raw values.
func BuildPlaceholders(vars map[string]string) map[string]string {
    placeholders := make(map[string]string)
    for key, value := range vars {
        placeholders[fmt.Sprintf("{{.%s}}", key)] = value
        placeholders[fmt.Sprintf("{{.PascalCase%s}}", key)] = ToPascalCase(value)
        placeholders[fmt.Sprintf("{{.CamelCase%s}}", key)] = ToCamelCase(value)
        placeholders[fmt.Sprintf("{{.KebabCase%s}}", key)] = ToKebabCase(value)
        placeholders[fmt.Sprintf("{{.LowerCase%s}}", key)] = strings.ToLower(value)
        placeholders[fmt.Sprintf("{{.UpperCase%s}}", key)] = strings.ToUpper(value)

        placeholders[fmt.Sprintf("{{ .%s }}", key)] = value
        placeholders[fmt.Sprintf("{{ .PascalCase%s }}", key)] = ToPascalCase(value)
        placeholders[fmt.Sprintf("{{ .CamelCase%s }}", key)] = ToCamelCase(value)
        placeholders[fmt.Sprintf("{{ .KebabCase%s }}", key)] = ToKebabCase(value)
        placeholders[fmt.Sprintf("{{ .LowerCase%s }}", key)] = strings.ToLower(value)
        placeholders[fmt.Sprintf("{{ .UpperCase%s }}", key)] = strings.ToUpper(value)
    }
    return placeholders
}

// BuildMultiPlaceholders builds a placeholder map with a main variable plus extras.
func BuildMultiPlaceholders(mainValue string, extraVars map[string]string) map[string]string {
    placeholders := BuildPlaceholders(map[string]string{"Main": mainValue})
    for key, value := range extraVars {
        extra := BuildPlaceholders(map[string]string{key: value})
        for k, v := range extra { placeholders[k] = v }
    }
    return placeholders
}

// BuildAutoPlaceholders builds a placeholder map from given variables.
func BuildAutoPlaceholders(vars map[string]string) map[string]string {
    if len(vars) == 1 {
        for k, value := range vars {
            if k == "Main" { return BuildPlaceholders(map[string]string{"Main": value}) }
            return BuildPlaceholders(vars)
        }
    }
    return BuildPlaceholders(vars)
}

// -----------------------------
// Inline fallback application for new files
// -----------------------------

// applyInlineFallbacksForNewFile applies inline fallback edits (insertBeforeInline/insertAfterInline
// and conditional replacements) to content for newly created files. This ensures first-run injections
// like SITEMAP TYPES are applied even when the file doesn't exist yet.
func applyInlineFallbacksForNewFile(content string, node TreeNode, placeholders map[string]string) string {
    if len(node.Markers) == 0 {
        return content
    }
    // Extract snippet groups from the template code to support snippet-based inline injections
    tmplSnippets, _ := extractSnippets(node.Code)

    for _, m := range node.Markers {
        mk := strings.TrimSpace(m.Mark)
        if mk == "" { continue }

        if m.Fallback.Spec != nil {
            beh := strings.ToLower(strings.TrimSpace(m.Fallback.Spec.Behaviour))

            // Handle conditional replace behaviours
            if beh == "replaceifmissing" || beh == "replaceifabsent" || beh == "replace" {
                tgt := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                rep := replacePlaceholders(m.Fallback.Spec.Replacement, placeholders)
                req := replacePlaceholders(m.Fallback.Spec.RequireAbsent, placeholders)
                occ := m.Fallback.Spec.Occurrence
                if modified, did := conditionalReplace(content, tgt, req, rep, occ); did {
                    content = modified
                }
                continue
            }

            // Block replacement between anchors for new files
            if beh == "replacebetween" || beh == "replaceblock" || beh == "replacerange" {
                start := replacePlaceholders(m.Fallback.Spec.TargetStart, placeholders)
                if strings.TrimSpace(start) == "" { start = replacePlaceholders(m.Fallback.Spec.Target, placeholders) }
                end := replacePlaceholders(m.Fallback.Spec.TargetEnd, placeholders)
                rep := replacePlaceholders(m.Fallback.Spec.Replacement, placeholders)
                req := replacePlaceholders(m.Fallback.Spec.RequireAbsent, placeholders)
                occ := m.Fallback.Spec.Occurrence
                if modified, did := replaceBetweenAnchors(content, start, end, req, rep, occ); did {
                    content = modified
                }
                continue
            }

            // Inline insertion behaviour (same-line)
            if strings.EqualFold(beh, "insertBeforeInline") || strings.EqualFold(beh, "insertAfterInline") {
                var snip string
                if strings.TrimSpace(m.Fallback.Spec.Content) != "" {
                    snip = replacePlaceholders(m.Fallback.Spec.Content, placeholders)
                } else {
                    // Try to find a snippet group matching the marker key
                    if s, ok := findSnippetForKeyGlobal(tmplSnippets, mk); ok {
                        snip = s
                    } else {
                        // No snippet to insert
                        continue
                    }
                }
                target := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                occurrence := m.Fallback.Spec.Occurrence
                behaviour := beh
                if behaviour == "" { behaviour = "insertBeforeInline" }
                if modified, inserted := insertSnippetInlineRelativeToTarget(content, snip, target, behaviour, occurrence); inserted {
                    content = modified
                }
                continue
            }
            // Line insertion behaviour (new line before/after target line)
            if strings.EqualFold(beh, "insertBeforeLine") || strings.EqualFold(beh, "insertAfterLine") || strings.EqualFold(beh, "insertNextLine") || m.Fallback.Spec.FallbackOnly {
                var snip string
                if strings.TrimSpace(m.Fallback.Spec.Content) != "" {
                    snip = replacePlaceholders(m.Fallback.Spec.Content, placeholders)
                } else {
                    if s, ok := findSnippetForKeyGlobal(tmplSnippets, mk); ok {
                        snip = s
                    } else { continue }
                }
                target := replacePlaceholders(m.Fallback.Spec.Target, placeholders)
                occurrence := m.Fallback.Spec.Occurrence
                behaviour := beh
                if behaviour == "" { behaviour = "insertAfterLine" }
                if modified, inserted := insertSnippetOnNewLineRelativeToTarget(content, snip, target, behaviour, occurrence); inserted {
                    content = modified
                }
                continue
            }
        }
    }
    return content
}

// -----------------------------
// Preview helpers (shared type)
// -----------------------------

// We import utils in this file to avoid duplicate import cycles; used by devtools too.
var _ = utils.RenderFileTree
