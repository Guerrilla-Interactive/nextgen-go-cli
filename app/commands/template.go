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
	Key       string     `json:"_key"`
	Type      string     `json:"_type"`
	Children  []TreeNode `json:"children"`
	Code      string     `json:"code"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	IsIndexer bool       `json:"isIndexer"` // even if false, we'll override if we see the marker in the code
	// New schema uses actions/title/logic. We also accept legacy markers/mark/fallback.
	Actions []InsertionAction `json:"actions"`
	Markers []InsertionAction `json:"markers"`
}

// InsertionAction describes a desired insertion and its logic.
// Legacy fields (mark/fallback) are kept for backward compatibility with older templates.
type InsertionAction struct {
	// New names
	Title string         `json:"title"`
	Logic MarkerFallback `json:"logic"`
	// Legacy aliases
	Mark     string         `json:"mark"`
	Fallback MarkerFallback `json:"fallback"`
}

// normalized returns a copy where Title/Logic are populated from legacy fields if missing.
func (a InsertionAction) normalized() InsertionAction {
	na := a
	if strings.TrimSpace(na.Title) == "" {
		na.Title = strings.TrimSpace(na.Mark)
	}
	if (na.Logic.Spec == nil && na.Logic.Raw == "") && (na.Fallback.Spec != nil || na.Fallback.Raw != "") {
		na.Logic = na.Fallback
	}
	return na
}

// getActions merges new Actions with legacy Markers and returns normalized actions.
func (n TreeNode) getActions() []InsertionAction {
	var out []InsertionAction
	if len(n.Actions) > 0 {
		for _, a := range n.Actions {
			out = append(out, a.normalized())
		}
	}
	if len(n.Markers) > 0 {
		for _, a := range n.Markers {
			out = append(out, a.normalized())
		}
	}
	return out
}

// MarkerFallback supports legacy string fallbacks and structured object fallbacks.
type MarkerFallback struct {
	Raw  string              // legacy fallback body
	Spec *MarkerFallbackSpec // structured fallback spec
}

type MarkerFallbackSpec struct {
	Target string `json:"target"`
	// Optional start/end anchors for block replacement
	TargetStart string `json:"targetStart"`
	TargetEnd   string `json:"targetEnd"`
	// Optional explicit marker key override to use instead of action title
	Mark string `json:"mark"`
	// Behaviour controls how logic applies. Supported values (case-insensitive):
	// - Marker insertion: addMarkerAboveTarget | addMarkerBelowTarget
	// - Inline insertion: insertBeforeInline | insertAfterInline
	// - Line insertion: insertBeforeLine | insertAfterLine
	// - Conditional replace: replaceIfMissing
	// - Anchored block replace: replaceBetween
	Behaviour     string `json:"behaviour"`
	Content       string `json:"content"`
	FallbackOnly  bool   `json:"fallbackOnly"`
	Occurrence    string `json:"occurrence"` // first | last
	RequireAbsent string `json:"requireAbsent"`
	Replacement   string `json:"replacement"`
}

// normalizeBehaviour maps various synonym behaviours to a single canonical form
// to reduce branching and duplication. Returned values are lowercase.
func normalizeBehaviour(beh string) string {
	b := strings.ToLower(strings.TrimSpace(beh))
	switch b {
	// Marker insertion (canonical only)
	case "addmarkerabovetarget", "addmarkerbelowtarget":
		return b
	// Inline insertion (canonical only)
	case "insertbeforeinline", "insertafterinline":
		return b
	// Line insertion (canonical only)
	case "insertbeforeline", "insertafterline":
		return b
	case "insertnextline":
		return "insertafterline"
	// Conditional replace (canonical only)
	case "replaceifmissing":
		return b
	// Block replace (canonical only)
	case "replacebetween":
		return b
	default:
		return b
	}
}

// UnmarshalJSON allows MarkerFallback to be a string or an object.
func (m *MarkerFallback) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if len(s) == 0 || s == "null" {
		return nil
	}
	if s[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		m.Raw = v
		m.Spec = nil
		return nil
	}
	if s[0] == '{' {
		var spec MarkerFallbackSpec
		if err := json.Unmarshal(b, &spec); err != nil {
			return err
		}
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
	if err != nil {
		return fmt.Errorf("could not read JSON template: %w", err)
	}
	var template JSONCommandTemplate
	if err := json.Unmarshal(templateBytes, &template); err != nil {
		return fmt.Errorf("could not parse JSON template: %w", err)
	}
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
			if err := os.MkdirAll(currentPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", currentPath, err)
			}
			if err := gatherNodes(node.Children, currentPath, projectPath, placeholders); err != nil {
				return err
			}
			continue
		}

		if node.Code == "" {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(currentPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", currentPath, err)
		}
		code := replacePlaceholders(node.Code, placeholders)

		// Detect indexer
		isIndexer := node.IsIndexer
		if !isIndexer {
			indexerMarker := regexp.MustCompile(`(?m)^\s*//\s*THIS\s+IS\s+AN\s+INDEXER\s+FILE`)
			if indexerMarker.FindStringIndex(code) != nil || strings.Contains(code, "// THIS IS AN INDEXER FILE") {
				isIndexer = true
				if cli.IsVerboseEnabled() {
					fmt.Printf("ℹ️  Detected indexer marker in file %s, registering as an indexer file.\n", currentPath)
				}
			}
		}
		if !isIndexer {
			if snippetMap, _ := extractSnippets(code); len(snippetMap) > 0 {
				isIndexer = true
				if cli.IsVerboseEnabled() {
					fmt.Printf("ℹ️  Treating %s as indexer based on presence of snippet groups.\n", currentPath)
				}
			}
		}

		if _, err := os.Stat(currentPath); err == nil {
			// Existing file
			if isIndexer {
				existingContentBytes, readErr := os.ReadFile(currentPath)
				if readErr != nil {
					return fmt.Errorf("failed to read existing file %s: %w", currentPath, readErr)
				}
				existingContent := string(existingContentBytes)

				// Ensure explicit node actions exist; insert via logic if missing.
				actions := node.getActions()
				if len(actions) > 0 {
					tmplSnippets, _ := extractSnippets(code)
					for _, m := range actions {
						nm := m.normalized()
						mk := strings.TrimSpace(nm.Title)
						if nm.Logic.Spec != nil && strings.TrimSpace(nm.Logic.Spec.Mark) != "" {
							mk = strings.TrimSpace(nm.Logic.Spec.Mark)
						}
						if mk == "" {
							continue
						}
						if nm.Logic.Spec != nil {
							beh := normalizeBehaviour(nm.Logic.Spec.Behaviour)
							if beh == "replaceifmissing" {
								tgt := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
								rep := replacePlaceholders(nm.Logic.Spec.Replacement, placeholders)
								req := replacePlaceholders(nm.Logic.Spec.RequireAbsent, placeholders)
								occ := nm.Logic.Spec.Occurrence
								if modified, did := conditionalReplace(existingContent, tgt, req, rep, occ); did {
									existingContent = modified
									if cli.IsVerboseEnabled() {
										fmt.Printf("✓ Replaced inline for '%s' in %s.\n", mk, currentPath)
									}
								}
								continue
							}
							// Replace a block between start/end anchors
							if beh == "replacebetween" {
								start := replacePlaceholders(nm.Logic.Spec.TargetStart, placeholders)
								if strings.TrimSpace(start) == "" {
									start = replacePlaceholders(nm.Logic.Spec.Target, placeholders)
								}
								end := replacePlaceholders(nm.Logic.Spec.TargetEnd, placeholders)
								rep := replacePlaceholders(nm.Logic.Spec.Replacement, placeholders)
								req := replacePlaceholders(nm.Logic.Spec.RequireAbsent, placeholders)
								occ := nm.Logic.Spec.Occurrence
								if modified, did := replaceBetweenAnchors(existingContent, start, end, req, rep, occ); did {
									existingContent = modified
									if cli.IsVerboseEnabled() {
										fmt.Printf("✓ Replaced block for '%s' in %s.\n", mk, currentPath)
									}
								}
								continue
							}
						}
						// Inline-injection (same-line)
						if nm.Logic.Spec != nil {
							beh := normalizeBehaviour(nm.Logic.Spec.Behaviour)
							if beh == "insertbeforeinline" || beh == "insertafterinline" {
								var snip string
								if strings.TrimSpace(nm.Logic.Spec.Content) != "" {
									snip = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
								} else {
									var ok bool
									snip, ok = findSnippetForKeyGlobal(tmplSnippets, mk)
									if !ok || strings.TrimSpace(snip) == "" {
										continue
									}
								}
								target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
								behaviour := normalizeBehaviour(beh)
								occurrence := nm.Logic.Spec.Occurrence
								if modified, inserted := insertSnippetInlineRelativeToTarget(existingContent, snip, target, behaviour, occurrence); inserted {
									existingContent = modified
									if cli.IsVerboseEnabled() {
										fmt.Printf("✓ Injected inline snippet for '%s' in %s.\n", mk, currentPath)
									}
								}
								continue
							}
						}
						// Line-injection (new line before/after target line)
						if nm.Logic.Spec != nil {
							beh := normalizeBehaviour(nm.Logic.Spec.Behaviour)
							if beh == "insertbeforeline" || beh == "insertafterline" || nm.Logic.Spec.FallbackOnly {
								var snip string
								if strings.TrimSpace(nm.Logic.Spec.Content) != "" {
									snip = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
								} else {
									var ok bool
									snip, ok = findSnippetForKeyGlobal(tmplSnippets, mk)
									if !ok || strings.TrimSpace(snip) == "" {
										continue
									}
								}
								target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
								behaviour := normalizeBehaviour(beh)
								occurrence := nm.Logic.Spec.Occurrence
								// Prefer to insert relative to an existing marker for this key
								var modified string
								var inserted bool
								if modM, insM := insertSnippetBelowMarker(existingContent, mk, snip, occurrence); insM {
									modified, inserted = modM, true
								} else if modT, insT := insertSnippetOnNewLineRelativeToTarget(existingContent, snip, target, behaviour, occurrence); insT {
									modified, inserted = modT, true
								}
								if inserted {
									existingContent = modified
									// Add a marker when not fallback-only, aligned with insertion direction
									if !nm.Logic.Spec.FallbackOnly && !markerForKeyExists(existingContent, mk) {
										markerBeh := "addmarkerbelowtarget"
										if behaviour == "insertbeforeline" {
											markerBeh = "addmarkerabovetarget"
										}
										if mod2, ins2 := insertAddMarkerRelativeToTarget(existingContent, mk, target, markerBeh, occurrence); ins2 {
											existingContent = mod2
										}
									}
									if cli.IsVerboseEnabled() {
										fmt.Printf("✓ Injected snippet on new line for '%s' in %s.\n", mk, currentPath)
									}
								}
								continue
							}
						}
						// Marker creation (line-based or fallback block)
						if !markerForKeyExists(existingContent, mk) {
							var modified string
							var inserted bool
							if nm.Logic.Raw != "" {
								modified, inserted = insertAddMarkerAfterFallback(existingContent, mk, replacePlaceholders(nm.Logic.Raw, placeholders))
							} else if nm.Logic.Spec != nil {
								target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
								behaviour := normalizeBehaviour(nm.Logic.Spec.Behaviour)
								occurrence := nm.Logic.Spec.Occurrence
								modified, inserted = insertAddMarkerRelativeToTarget(existingContent, mk, target, behaviour, occurrence)
							}
							if inserted {
								existingContent = modified
								if cli.IsVerboseEnabled() {
									fmt.Printf("ℹ️  Inserted missing marker for '%s' in %s using fallback.\n", mk, currentPath)
								}
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
								if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
									b.WriteByte(c)
								}
							}
							return b.String()
						}
						var skipMarks []string
						for _, mm := range node.getActions() {
							nm := mm.normalized()
							if nm.Logic.Spec != nil && (strings.Contains(strings.ToLower(nm.Logic.Spec.Behaviour), "inline") || nm.Logic.Spec.FallbackOnly) {
								mks := strings.TrimSpace(nm.Title)
								if nm.Logic.Spec != nil && strings.TrimSpace(nm.Logic.Spec.Mark) != "" {
									mks = strings.TrimSpace(nm.Logic.Spec.Mark)
								}
								if mks != "" {
									skipMarks = append(skipMarks, sanitize(mks))
								}
							}
						}
						var keys []string
						for k := range snippetMap {
							ks := sanitize(k)
							skip := false
							for _, sm := range skipMarks {
								if ks == sm || strings.Contains(ks, sm) || strings.Contains(sm, ks) {
									skip = true
									break
								}
							}
							if !skip {
								keys = append(keys, k)
							}
						}
						if len(keys) > 0 {
							if modified, inserted := autoInsertIndexerMarkers(existingContent, keys); inserted {
								existingContent = modified
								if cli.IsVerboseEnabled() {
									fmt.Printf("ℹ️  Inserted %d indexer markers into %s.\n", len(keys), currentPath)
								}
							}
						}
					}
				}

				// Merge template snippets (augmented with missing fallback snippets)
				codeWithFallbackSnippets := augmentTemplateWithFallbackSnippets(code, node.getActions(), placeholders)
				mergedContent, mergeErr := smartMerge(existingContent, codeWithFallbackSnippets)
				if mergeErr != nil {
					return fmt.Errorf("failed to merge file %s: %w", currentPath, mergeErr)
				}
				mergedContent = cleanupIndexerContent(mergedContent)
				mergedContent = ensureExportForLinkReference(mergedContent)
				if err := os.WriteFile(currentPath, []byte(mergedContent), 0644); err != nil {
					return fmt.Errorf("failed to write merged file %s: %w", currentPath, err)
				}
				if cli.IsVerboseEnabled() {
					fmt.Printf("✓ Merged updates into existing file %s.\n", currentPath)
				}
				if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
					MarkEditedIndexer(rel)
				} else {
					MarkEditedIndexer(currentPath)
				}
			} else {
				// Non-indexer overwrite
				newContent := removeSnippetMarkers(code)
				if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil {
					return fmt.Errorf("failed to overwrite file %s: %w", currentPath, err)
				}
				if cli.IsVerboseEnabled() {
					fmt.Printf("✓ Replaced existing file %s.\n", currentPath)
				}
			}
			if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
				RecordCreatedFile(rel)
			} else {
				RecordCreatedFile(currentPath)
			}
			continue
		}

		// New file
		if isIndexer {
			newContent := removeSnippetMarkers(code)
			// Apply inline fallback injections (e.g., insertBeforeInline) for brand new files
			newContent = applyInlineFallbacksForNewFile(newContent, node, placeholders)
			newContent = cleanupIndexerContent(newContent)
			newContent = ensureExportForLinkReference(newContent)
			if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("failed to write new indexer file %s: %w", currentPath, err)
			}
			if cli.IsVerboseEnabled() {
				fmt.Printf("✓ Created new indexer file %s.\n", currentPath)
			}
		} else {
			newContent := removeSnippetMarkers(code)
			// Apply inline fallback injections (e.g., insertBeforeInline) for brand new files
			newContent = applyInlineFallbacksForNewFile(newContent, node, placeholders)
			newContent = ensureExportForLinkReference(newContent)
			if err := os.WriteFile(currentPath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", currentPath, err)
			}
		}
		if rel, err := filepath.Rel(projectPath, currentPath); err == nil {
			RecordCreatedFile(rel)
		} else {
			RecordCreatedFile(currentPath)
		}
	}
	return nil
}

// replacePlaceholders walks through the placeholders map and replaces all occurrences.
func replacePlaceholders(content string, placeholders map[string]string) string {
	for oldVal, newVal := range placeholders {
		content = strings.ReplaceAll(content, oldVal, newVal)
	}
	return content
}

// RunJsonTemplate loads and executes a command template from a JSON file.
func RunJsonTemplate(jsonFilePath, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplate(jsonFilePath, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template: %w", err)
	}
	return nil
}

// RunJsonTemplateBytes loads and executes a command template from byte data.
func RunJsonTemplateBytes(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	if err := ExecuteJSONTemplateFromMemory(jsonBytes, projectPath, placeholders); err != nil {
		return fmt.Errorf("failed to run JSON template from memory: %w", err)
	}
	return nil
}

// ExecuteJSONTemplateFromMemory executes the template logic given the JSON bytes.
func ExecuteJSONTemplateFromMemory(jsonBytes []byte, projectPath string, placeholders map[string]string) error {
	var template JSONCommandTemplate
	if err := json.Unmarshal(jsonBytes, &template); err != nil {
		return fmt.Errorf("could not parse JSON template: %w", err)
	}
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

// Placeholder transforms and usage examples
//
// Transforms are case-insensitive and support short names. You can also add spaces
// inside the moustaches. All of these are equivalent forms:
//   {{.KebabCaseName}}, {{.KebabName}}, {{.KEBABName}}, {{ .kebabName }}
//   {{.PascalCaseName}}, {{.PascalName}}, {{.PASCALName}}, {{ .pascalName }}
//   {{.CamelCaseName}},  {{.CamelName}},  {{.CAMELName}},  {{ .camelName }}
//   {{.SnakeCaseName}},  {{.SnakeName}},  {{.SNAKEName}},  {{ .snakeName }}
//   {{.ScreamingSnakeCaseName}}, {{.ScreamingSnakeName}}, {{.SCREAMINGSNAKEName}}, {{ .screamingSnakeName }}
//   {{.UpperCaseName}},  {{.UpperName}},  {{.UPPERName}},  {{ .upperName }}
//   {{.LowerCaseName}},  {{.LowerName}},  {{.LOWERName}},  {{ .lowerName }}
//
// Given: Name = "My great_value-Here"
// Outputs:
// - Raw:              {{.Name}}                        -> "My great_value-Here"
// - Pascal:           {{.PascalCaseName}}              -> "MyGreat_valueHere"
// - Camel:            {{.CamelCaseName}}               -> "myGreat_valueHere"
// - Kebab:            {{.KebabCaseName}}               -> "my-great_value-here"
// - Snake:            {{.SnakeCaseName}}               -> "my_great_value_here"
// - ScreamingSnake:   {{.ScreamingSnakeCaseName}}      -> "MY_GREAT_VALUE_HERE"
// - Upper:            {{.UpperCaseName}}               -> "MY GREAT_VALUE-HERE"
// - Lower:            {{.LowerCaseName}}               -> "my great_value-here"
//
// Notes:
// - Hyphens ("-") and spaces split words. Underscores ("_") are preserved inside words
//   for Pascal/Camel/Kebab; Snake/ScreamingSnake join with underscores.
// - You can mix any transform casing (e.g., "KEBAB", "kebab", "KebabCase").

// ToKebabCase converts a string to kebab-case.
func ToKebabCase(input string) string {
	words := splitIntoWords(input)
	return strings.ToLower(strings.Join(words, "-"))
}

// ToPascalCase converts input to PascalCase.
func ToPascalCase(input string) string {
	words := splitIntoWords(input)
	for i, w := range words {
		words[i] = strings.Title(strings.ToLower(w))
	}
	return strings.Join(words, "")
}

// ToCamelCase converts input to camelCase.
func ToCamelCase(input string) string {
	pascal := ToPascalCase(input)
	if len(pascal) == 0 {
		return pascal
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// ToLowercase converts input to all lowercase.
func ToLowercase(input string) string { return strings.ToLower(input) }

// ToSnakeCase converts input to snake_case.
func ToSnakeCase(input string) string {
	words := splitIntoWords(input)
	return strings.ToLower(strings.Join(words, "_"))
}

// ToScreamingSnakeCase converts input to SCREAMING_SNAKE_CASE.
func ToScreamingSnakeCase(input string) string {
	words := splitIntoWords(input)
	return strings.ToUpper(strings.Join(words, "_"))
}

// splitIntoWords splits a string into words based on hyphens or spaces.
func splitIntoWords(s string) []string { s = strings.ReplaceAll(s, "-", " "); return strings.Fields(s) }

// splitCamelWords splits PascalCase/CamelCase tokens into words at uppercase boundaries.
func splitCamelWords(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	var curr []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' && len(curr) > 0 {
			words = append(words, string(curr))
			curr = []rune{r}
		} else {
			curr = append(curr, r)
		}
	}
	if len(curr) > 0 {
		words = append(words, string(curr))
	}
	return words
}

// getTransformNameVariants returns a set of acceptable casing variants for a
// transform token (e.g., "UpperCase", "UPPERCASE", "uppercase"). For Upper/Lower
// tokens, we also support short aliases like "upper"/"lower" in common casings.
func getTransformNameVariants(name string) []string {
	base := strings.TrimSpace(name)
	if base == "" {
		return []string{""}
	}

	variantsSet := map[string]struct{}{}
	add := func(v string) { variantsSet[v] = struct{}{} }

	// Canonical forms
	add(base)
	add(strings.ToUpper(base))
	add(strings.ToLower(base))
	// Title-case variant (e.g., "Uppercase")
	add(strings.Title(strings.ToLower(base)))
	// lowerCamel variant (e.g., "upperCase")
	if len(base) > 0 {
		add(strings.ToLower(base[:1]) + base[1:])
	}

	bl := strings.ToLower(base)
	if bl == "uppercase" || bl == "upper" {
		for _, alias := range []string{"upper", "UPPER", "Upper"} {
			add(alias)
		}
	}
	if bl == "lowercase" || bl == "lower" {
		for _, alias := range []string{"lower", "LOWER", "Lower"} {
			add(alias)
		}
	}
	if bl == "pascalcase" || bl == "pascal" {
		for _, alias := range []string{"pascal", "PASCAL", "Pascal"} {
			add(alias)
		}
	}
	if bl == "camelcase" || bl == "camel" {
		for _, alias := range []string{"camel", "CAMEL", "Camel"} {
			add(alias)
		}
	}
	if bl == "kebabcase" || bl == "kebab" {
		for _, alias := range []string{"kebab", "KEBAB", "Kebab"} {
			add(alias)
		}
	}
	if bl == "snakecase" || bl == "snake" {
		for _, alias := range []string{"snake", "SNAKE", "Snake"} {
			add(alias)
		}
	}
	if bl == "screamingsnakecase" || bl == "screamingsnake" {
		for _, alias := range []string{"screamingsnake", "SCREAMINGSNAKE", "ScreamingSnake"} {
			add(alias)
		}
	}

	// Underscore- and hyphen-separated aliases for multi-word tokens (e.g., ScreamingSnake -> SCREAMING_SNAKE / screaming-snake)
	existing := make([]string, 0, len(variantsSet))
	for v := range variantsSet {
		existing = append(existing, v)
	}
	for _, tok := range existing {
		parts := splitCamelWords(tok)
		if len(parts) > 1 {
			joinedUnderscore := strings.Join(parts, "_")
			add(joinedUnderscore)
			add(strings.ToLower(joinedUnderscore))
			add(strings.ToUpper(joinedUnderscore))
			joinedHyphen := strings.Join(parts, "-")
			add(joinedHyphen)
			add(strings.ToLower(joinedHyphen))
			add(strings.ToUpper(joinedHyphen))
		}
	}

	variants := make([]string, 0, len(variantsSet))
	for v := range variantsSet {
		variants = append(variants, v)
	}
	return variants
}

// addPlaceholderForTransform registers placeholder keys for a specific transform
// token and variable key using multiple casing variants, with and without inner
// spaces. Examples: {{.UpperCaseName}}, {{.UPPERCASEName}}, {{ .uppercaseName }}
// getVariableNameVariants generates forgiving variants for the variable key part
// (e.g., PageType -> PAGE_TYPE, pagetype, page_type, etc.).
func getVariableNameVariants(key string) []string {
	base := strings.TrimSpace(key)
	if base == "" {
		return []string{""}
	}
	set := map[string]struct{}{}
	add := func(v string) {
		if strings.TrimSpace(v) != "" {
			set[v] = struct{}{}
		}
	}

	// Original token as provided
	add(base)

	// Build words: consider hyphens/underscores/spaces and camel/pascal boundaries
	spaced := strings.NewReplacer("-", " ", "_", " ").Replace(base)
	words := strings.Fields(spaced)
	if len(words) <= 1 {
		// Try camel/pascal split when single token
		cw := splitCamelWords(base)
		if len(cw) > 1 {
			words = cw
		}
	}
	// Normalize words to lowercase for kebab/snake
	lowerWords := make([]string, 0, len(words))
	for _, w := range words {
		lw := strings.ToLower(strings.TrimSpace(w))
		if lw != "" {
			lowerWords = append(lowerWords, lw)
		}
	}

	if len(lowerWords) > 0 {
		// PascalCase
		var pasBuilder strings.Builder
		for _, lw := range lowerWords {
			pasBuilder.WriteString(strings.Title(lw))
		}
		pas := pasBuilder.String()
		add(pas)
		// camelCase
		if pas != "" {
			add(strings.ToLower(pas[:1]) + pas[1:])
		}
		// snake_case and SCREAMING_SNAKE_CASE
		snk := strings.Join(lowerWords, "_")
		add(snk)
		add(strings.ToUpper(snk))
		// kebab-case
		keb := strings.Join(lowerWords, "-")
		add(keb)
		// Compact upper/lower (PAGETYPE/pagetype)
		compact := strings.ReplaceAll(snk, "_", "")
		add(strings.ToUpper(compact))
		add(strings.ToLower(compact))
	}

	// Also include lowercase base
	add(strings.ToLower(base))

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}

func addPlaceholderForTransform(placeholders map[string]string, transformToken string, variableKey string, value string) {
	varKeys := getVariableNameVariants(variableKey)
	for _, t := range getTransformNameVariants(transformToken) {
		for _, vk := range varKeys {
			if t == "" {
				placeholders[fmt.Sprintf("{{.%s}}", vk)] = value
				placeholders[fmt.Sprintf("{{ .%s }}", vk)] = value
				continue
			}
			// Concatenated form
			placeholders[fmt.Sprintf("{{.%s%s}}", t, vk)] = value
			placeholders[fmt.Sprintf("{{ .%s%s }}", t, vk)] = value
			// Underscore between transform and key
			placeholders[fmt.Sprintf("{{.%s_%s}}", t, vk)] = value
			placeholders[fmt.Sprintf("{{ .%s_%s }}", t, vk)] = value
			// Hyphen between transform and key
			placeholders[fmt.Sprintf("{{.%s-%s}}", t, vk)] = value
			placeholders[fmt.Sprintf("{{ .%s-%s }}", t, vk)] = value
		}
	}
}

// BuildPlaceholders creates a map of placeholder variables from raw values.
func BuildPlaceholders(vars map[string]string) map[string]string {
	placeholders := make(map[string]string)
	for key, value := range vars {
		// Raw variable (no transform token)
		addPlaceholderForTransform(placeholders, "", key, value)
		// PascalCase variants
		addPlaceholderForTransform(placeholders, "PascalCase", key, ToPascalCase(value))
		// CamelCase variants
		addPlaceholderForTransform(placeholders, "CamelCase", key, ToCamelCase(value))
		// KebabCase variants
		addPlaceholderForTransform(placeholders, "KebabCase", key, ToKebabCase(value))
		// SnakeCase variants
		addPlaceholderForTransform(placeholders, "SnakeCase", key, ToSnakeCase(value))
		addPlaceholderForTransform(placeholders, "Snake", key, ToSnakeCase(value))
		// ScreamingSnakeCase variants
		addPlaceholderForTransform(placeholders, "ScreamingSnakeCase", key, ToScreamingSnakeCase(value))
		addPlaceholderForTransform(placeholders, "ScreamingSnake", key, ToScreamingSnakeCase(value))
		// LowerCase variants (and aliases)
		lowerVal := strings.ToLower(value)
		addPlaceholderForTransform(placeholders, "LowerCase", key, lowerVal)
		addPlaceholderForTransform(placeholders, "lower", key, lowerVal)
		// UpperCase variants (and aliases)
		upperVal := strings.ToUpper(value)
		addPlaceholderForTransform(placeholders, "UpperCase", key, upperVal)
		addPlaceholderForTransform(placeholders, "upper", key, upperVal)
	}
	return placeholders
}

// BuildMultiPlaceholders builds a placeholder map with a main variable plus extras.
func BuildMultiPlaceholders(mainValue string, extraVars map[string]string) map[string]string {
	placeholders := BuildPlaceholders(map[string]string{"Main": mainValue})
	for key, value := range extraVars {
		extra := BuildPlaceholders(map[string]string{key: value})
		for k, v := range extra {
			placeholders[k] = v
		}
	}
	return placeholders
}

// BuildAutoPlaceholders builds a placeholder map from given variables.
func BuildAutoPlaceholders(vars map[string]string) map[string]string {
	if len(vars) == 1 {
		for k, value := range vars {
			if k == "Main" {
				return BuildPlaceholders(map[string]string{"Main": value})
			}
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
	actions := node.getActions()
	if len(actions) == 0 {
		return content
	}
	// Normalize slug alias forms so fallback logic treats `slug.current as slug`
	// and `"slug": slug.current` as equivalent when scanning content.
	content = canonicalizeSlugAliases(content)
	// Extract snippet groups from the template code to support snippet-based inline injections
	tmplSnippets, _ := extractSnippets(node.Code)

	for _, m := range actions {
		nm := m.normalized()
		mk := strings.TrimSpace(nm.Title)
		if nm.Logic.Spec != nil && strings.TrimSpace(nm.Logic.Spec.Mark) != "" {
			mk = strings.TrimSpace(nm.Logic.Spec.Mark)
		}
		if mk == "" {
			continue
		}

		if nm.Logic.Spec != nil {
			beh := normalizeBehaviour(nm.Logic.Spec.Behaviour)

			// Handle conditional replace behaviours
			if beh == "replaceifmissing" {
				tgt := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
				rep := replacePlaceholders(nm.Logic.Spec.Replacement, placeholders)
				req := replacePlaceholders(nm.Logic.Spec.RequireAbsent, placeholders)
				occ := nm.Logic.Spec.Occurrence
				if modified, did := conditionalReplace(content, tgt, req, rep, occ); did {
					content = modified
				}
				continue
			}

			// Block replacement between anchors for new files
			if beh == "replacebetween" {
				start := replacePlaceholders(nm.Logic.Spec.TargetStart, placeholders)
				if strings.TrimSpace(start) == "" {
					start = replacePlaceholders(nm.Logic.Spec.Target, placeholders)
				}
				end := replacePlaceholders(nm.Logic.Spec.TargetEnd, placeholders)
				rep := replacePlaceholders(nm.Logic.Spec.Replacement, placeholders)
				req := replacePlaceholders(nm.Logic.Spec.RequireAbsent, placeholders)
				occ := nm.Logic.Spec.Occurrence
				if modified, did := replaceBetweenAnchors(content, start, end, req, rep, occ); did {
					content = modified
				}
				continue
			}

			// Inline insertion behaviour (same-line)
			if beh == "insertbeforeinline" || beh == "insertafterinline" {
				var snip string
				if strings.TrimSpace(nm.Logic.Spec.Content) != "" {
					snip = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
				} else {
					// Try to find a snippet group matching the marker key
					if s, ok := findSnippetForKeyGlobal(tmplSnippets, mk); ok {
						snip = s
					} else {
						// No snippet to insert
						continue
					}
				}
				target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
				occurrence := nm.Logic.Spec.Occurrence
				behaviour := normalizeBehaviour(beh)
				if modified, inserted := insertSnippetInlineRelativeToTarget(content, snip, target, behaviour, occurrence); inserted {
					content = modified
				}
				continue
			}
			// Line insertion behaviour (new line before/after target line)
			if beh == "insertbeforeline" || beh == "insertafterline" || nm.Logic.Spec.FallbackOnly {
				var snip string
				if strings.TrimSpace(nm.Logic.Spec.Content) != "" {
					snip = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
				} else {
					if s, ok := findSnippetForKeyGlobal(tmplSnippets, mk); ok {
						snip = s
					} else {
						continue
					}
				}
				target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
				occurrence := nm.Logic.Spec.Occurrence
				behaviour := normalizeBehaviour(beh)
				if modified, inserted := insertSnippetOnNewLineRelativeToTarget(content, snip, target, behaviour, occurrence); inserted {
					content = modified
					// Add a marker when not fallback-only, aligned with insertion direction
					if !nm.Logic.Spec.FallbackOnly && !markerForKeyExists(content, mk) {
						markerBeh := "addmarkerbelowtarget"
						if behaviour == "insertbeforeline" {
							markerBeh = "addmarkerabovetarget"
						}
						if mod2, ins2 := insertAddMarkerRelativeToTarget(content, mk, target, markerBeh, occurrence); ins2 {
							content = mod2
						}
					}
				}
				continue
			}
		}
		// Marker behaviour (new file): inject snippet relative to target
		if nm.Logic.Spec != nil {
			beh2 := normalizeBehaviour(nm.Logic.Spec.Behaviour)
			if beh2 == "addmarkerabovetarget" || beh2 == "addmarkerbelowtarget" {
				target := replacePlaceholders(nm.Logic.Spec.Target, placeholders)
				occurrence := nm.Logic.Spec.Occurrence
				// When not fallback-only: insert snippet content
				if !nm.Logic.Spec.FallbackOnly {
					var snip string
					if strings.TrimSpace(nm.Logic.Spec.Content) != "" {
						snip = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
					} else {
						if s, ok := findSnippetForKeyGlobal(tmplSnippets, mk); ok {
							snip = s
						} else {
							// no snippet to insert; still can attempt marker if later needed
							snip = ""
						}
					}
					if strings.TrimSpace(snip) != "" {
						insertBeh := "insertbeforeline"
						if beh2 == "addmarkerbelowtarget" {
							insertBeh = "insertafterline"
						}
						if modified, inserted := insertSnippetOnNewLineRelativeToTarget(content, snip, target, insertBeh, occurrence); inserted {
							content = modified
							// Add marker aligned with insertion direction
							markerBeh := "addmarkerbelowtarget"
							if insertBeh == "insertbeforeline" {
								markerBeh = "addmarkerabovetarget"
							}
							if modified2, inserted2 := insertAddMarkerRelativeToTarget(content, mk, target, markerBeh, occurrence); inserted2 {
								content = modified2
							}
						}
					}
					continue
				}
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
