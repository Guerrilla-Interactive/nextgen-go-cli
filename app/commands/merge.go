package commands

import (
	"fmt"
	"regexp"
	"strings"
)

// -----------------------------------------------------------------------------
// [MERGE] Marker/snippet primitives & merge engine
// -----------------------------------------------------------------------------

// Global regex patterns for snippet markers.
var (
	startMarkerRegex = regexp.MustCompile(`(?m)^\s*//\s*START\s+OF\s+(.+)$`)
	endMarkerRegex   = regexp.MustCompile(`(?m)^\s*//\s*END\s+OF\s+(.+)$`)
	addMarkerRegex   = regexp.MustCompile(`(?m)^\s*//\s*ADD\s+(.+?)\s+(BELOW|ABOVE)\s*$`)
)

// hasAnySnippetMarkers returns true if the content already includes any snippet
// markers (START/END/ADD). Used to avoid inserting duplicates.
func hasAnySnippetMarkers(content string) bool {
	return startMarkerRegex.MatchString(content) || endMarkerRegex.MatchString(content) || addMarkerRegex.MatchString(content)
}

// autoInsertIndexerMarkers heuristically inserts insertion markers into an
// existing indexer file that lacks them.
func autoInsertIndexerMarkers(existingContent string, snippetKeys []string) (string, bool) {
	if len(snippetKeys) == 0 {
		return existingContent, false
	}
	if hasAnySnippetMarkers(existingContent) {
		return existingContent, false
	}

	lines := strings.Split(existingContent, "\n")

	importRegex := regexp.MustCompile(`^\s*(import\s|const\s+\w+\s*=\s*require\(|var\s+\w+\s*=\s*require\()`) // JS/TS common
	exportRegex := regexp.MustCompile(`^\s*(export\s|module\.exports\s*=|exports\.)`)                         // JS/TS common (default|const|named)
	listItemRegex := regexp.MustCompile(`^\s*[_$a-zA-Z][_$a-zA-Z0-9]*\s*,\s*$`)                               // e.g. "  myType,"

	lastImportIdx := -1
	firstExportIdx := -1
	lastListItemIdx := -1
	for i, ln := range lines {
		if importRegex.MatchString(ln) {
			lastImportIdx = i
		}
		if firstExportIdx == -1 && exportRegex.MatchString(ln) {
			firstExportIdx = i
		}
		if listItemRegex.MatchString(ln) {
			lastListItemIdx = i
		}
	}

	// Group keys by intent
	var importKeys, exportKeys, tailKeys []string
	for _, k := range snippetKeys {
		kl := strings.ToLower(strings.TrimSpace(k))
		switch {
		case strings.Contains(kl, "import"):
			importKeys = append(importKeys, k)
		case strings.Contains(kl, "export"):
			exportKeys = append(exportKeys, k)
		default:
			tailKeys = append(tailKeys, k)
		}
	}

	inserted := 0

	// Helper to insert a line into a slice at index i
	insertLine := func(sl []string, idx int, val string) []string {
		if idx < 0 {
			idx = 0
		}
		if idx > len(sl) {
			idx = len(sl)
		}
		sl = append(sl[:idx], append([]string{val}, sl[idx:]...)...)
		return sl
	}

	// After last import
	if lastImportIdx >= 0 && len(importKeys) > 0 {
		pos := lastImportIdx + 1
		for _, k := range importKeys {
			marker := fmt.Sprintf("// ADD %s BELOW", k)
			lines = insertLine(lines, pos, marker)
			pos++
			inserted++
		}
	}

	// Above first export
	if firstExportIdx >= 0 && len(exportKeys) > 0 {
		pos := firstExportIdx // ABOVE => insert before
		for _, k := range exportKeys {
			marker := fmt.Sprintf("// ADD %s ABOVE", k)
			lines = insertLine(lines, pos, marker)
			pos++
			inserted++
		}
	}

	// Place tail keys near likely list of exported items, else above export, else at EOF
	if len(tailKeys) > 0 {
		if lastListItemIdx >= 0 {
			pos := lastListItemIdx + 1
			for _, k := range tailKeys {
				marker := fmt.Sprintf("// ADD %s BELOW", k)
				lines = insertLine(lines, pos, marker)
				pos++
				inserted++
			}
		} else if firstExportIdx >= 0 {
			pos := firstExportIdx
			for _, k := range tailKeys {
				marker := fmt.Sprintf("// ADD %s ABOVE", k)
				lines = insertLine(lines, pos, marker)
				pos++
				inserted++
			}
		} else {
			// Ensure file ends with a newline for cleaner appends
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				lines = append(lines, "")
			}
			for _, k := range tailKeys {
				marker := fmt.Sprintf("// ADD %s BELOW", k)
				lines = append(lines, marker)
				inserted++
			}
		}
	}

	if inserted == 0 {
		return existingContent, false
	}
	return strings.Join(lines, "\n"), true
}

// markerForKeyExists returns true if there is already an ADD marker for the given key
// in the existing content (either ABOVE or BELOW).
func markerForKeyExists(content, key string) bool {
	pattern := regexp.MustCompile(`(?m)^\s*//\s*ADD\s+` + regexp.QuoteMeta(key) + `\s+(?:BELOW|ABOVE)\s*$`)
	return pattern.FindStringIndex(content) != nil
}

// insertAddMarkerAfterFallback finds fallback block and inserts marker after it.
func insertAddMarkerAfterFallback(existingContent, key, fallback string) (string, bool) {
	if strings.TrimSpace(fallback) == "" {
		return existingContent, false
	}

	lines := strings.Split(existingContent, "\n")
	want := strings.Split(strings.TrimSuffix(fallback, "\n"), "\n")
	for len(want) > 0 && strings.TrimSpace(want[len(want)-1]) == "" {
		want = want[:len(want)-1]
	}
	if len(want) == 0 {
		return existingContent, false
	}

	lastMatchEnd := -1
	lastIndent := ""
	for i := 0; i+len(want) <= len(lines); i++ {
		matched := true
		for j := 0; j < len(want); j++ {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(want[j]) {
				matched = false
				break
			}
		}
		if matched {
			lastMatchEnd = i + len(want) - 1
			ln := lines[lastMatchEnd]
			idx := 0
			for idx < len(ln) && (ln[idx] == ' ' || ln[idx] == '\t') {
				idx++
			}
			lastIndent = ln[:idx]
		}
	}
	if lastMatchEnd == -1 {
		// Avoid hard-coded special cases. If fallback block isn't found,
		// default to appending a marker at a sensible location (below the last
		// non-empty line, preserving indentation).
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				ln := lines[i]
				idx := 0
				for idx < len(ln) && (ln[idx] == ' ' || ln[idx] == '\t') {
					idx++
				}
				lastIndent = ln[:idx]
				break
			}
		}
		marker := lastIndent + "// ADD " + key + " BELOW"
		lines = append(lines, marker)
		return strings.Join(lines, "\n"), true
	}

	marker := lastIndent + "// ADD " + key + " BELOW"
	insertAt := lastMatchEnd + 1
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}
	lines = append(lines[:insertAt], append([]string{marker}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n"), true
}

// insertAddMarkerRelativeToTarget inserts an ADD marker relative to a target line.
func insertAddMarkerRelativeToTarget(existingContent, key, target, behaviour, occurrence string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return existingContent, false
	}
	// Normalize to canonical marker behaviours
	behLower := strings.ToLower(strings.TrimSpace(behaviour))
	if behLower != "addmarkerabovetarget" && behLower != "addmarkerbelowtarget" {
		return existingContent, false
	}
	lines := strings.Split(existingContent, "\n")
	anchorIdx := -1
	var matches []int
	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], target) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return existingContent, false
	}
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	if occ == "first" {
		anchorIdx = matches[0]
	} else {
		anchorIdx = matches[len(matches)-1]
	}
	ln := lines[anchorIdx]
	j := 0
	for j < len(ln) && (ln[j] == ' ' || ln[j] == '\t') {
		j++
	}
	indent := ln[:j]
	marker := indent + "// ADD " + key + " "
	if behLower == "addmarkerbelowtarget" {
		marker += "BELOW"
		insertAt := anchorIdx + 1
		if insertAt < 0 {
			insertAt = 0
		}
		if insertAt > len(lines) {
			insertAt = len(lines)
		}
		lines = append(lines[:insertAt], append([]string{marker}, lines[insertAt:]...)...)
	} else {
		marker += "ABOVE"
		insertAt := anchorIdx
		if insertAt < 0 {
			insertAt = 0
		}
		if insertAt > len(lines) {
			insertAt = len(lines)
		}
		lines = append(lines[:insertAt], append([]string{marker}, lines[insertAt:]...)...)
	}
	return strings.Join(lines, "\n"), true
}

// findSnippetForKeyGlobal exposes fuzzy snippet lookup used by smartMerge.
func findSnippetForKeyGlobal(snippetMap map[string]string, key string) (string, bool) {
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
	if sn, ok := snippetMap[key]; ok && strings.TrimSpace(sn) != "" {
		return sn, true
	}
	target := sanitize(key)
	type cand struct {
		k  string
		sn string
	}
	var contains []cand
	for sk, sn := range snippetMap {
		if strings.TrimSpace(sn) == "" {
			continue
		}
		skSan := sanitize(sk)
		if skSan == target {
			return sn, true
		}
		if strings.Contains(skSan, target) {
			contains = append(contains, cand{sk, sn})
		}
	}
	if len(contains) == 1 {
		return contains[0].sn, true
	}
	if len(contains) > 1 {
		best := contains[0]
		for _, c := range contains[1:] {
			if len(c.k) < len(best.k) {
				best = c
			}
		}
		return best.sn, true
	}
	for sk, sn := range snippetMap {
		if strings.TrimSpace(sn) == "" {
			continue
		}
		skSan := sanitize(sk)
		if strings.Contains(target, skSan) {
			return sn, true
		}
	}
	return "", false
}

// insertSnippetInlineRelativeToTarget inserts snippet inline before/after target text.
func insertSnippetInlineRelativeToTarget(existingContent, snippet, target, behaviour, occurrence string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || strings.TrimSpace(snippet) == "" {
		return existingContent, false
	}
	compressWS := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
		fields := strings.Fields(s)
		return strings.Join(fields, " ")
	}
	snippetInline := compressWS(snippet)
	behaviour = strings.ToLower(strings.TrimSpace(behaviour))
	if behaviour != "insertbeforeinline" && behaviour != "insertafterinline" {
		behaviour = "insertbeforeinline"
	}
	lines := strings.Split(existingContent, "\n")
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	anchorLine := -1
	anchorCol := -1
	if occ == "first" {
		for i := 0; i < len(lines); i++ {
			if idx := strings.Index(lines[i], target); idx >= 0 {
				anchorLine = i
				anchorCol = idx
				break
			}
		}
	} else {
		for i := 0; i < len(lines); i++ {
			if idx := strings.LastIndex(lines[i], target); idx >= 0 {
				anchorLine = i
				anchorCol = idx
			}
		}
	}
	if anchorLine == -1 {
		return existingContent, false
	}
	line := lines[anchorLine]
	if behaviour == "insertbeforeinline" {
		before := line[:anchorCol]
		if strings.Contains(before, snippetInline) {
			return existingContent, false
		}
		sep := ""
		if anchorCol > 0 && !strings.HasSuffix(before, " ") {
			sep = " "
		}
		line = before + sep + snippetInline + " " + line[anchorCol:]
	} else {
		afterIdx := anchorCol + len(target)
		after := line[afterIdx:]
		if strings.Contains(after, snippetInline) {
			return existingContent, false
		}
		sep := ""
		if len(after) > 0 && !strings.HasPrefix(after, " ") {
			sep = " "
		}
		line = line[:afterIdx] + sep + snippetInline + " " + after
	}
	lines[anchorLine] = line
	return strings.Join(lines, "\n"), true
}

// insertSnippetOnNewLineRelativeToTarget inserts snippet on a new line before/after
// the line that contains the target text. It preserves the indentation of the
// anchor line for the inserted block. Occurrence can be "first" or anything else
// (treated as "last").
func insertSnippetOnNewLineRelativeToTarget(existingContent, snippet, target, behaviour, occurrence string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || strings.TrimSpace(snippet) == "" {
		return existingContent, false
	}
	behaviour = strings.ToLower(strings.TrimSpace(behaviour))
	if behaviour == "insertnextline" {
		behaviour = "insertafterline"
	}
	if behaviour != "insertbeforeline" && behaviour != "insertafterline" {
		behaviour = "insertafterline"
	}
	lines := strings.Split(existingContent, "\n")
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	anchorLine := -1
	if occ == "first" {
		for i := 0; i < len(lines); i++ {
			if strings.Contains(lines[i], target) {
				anchorLine = i
				break
			}
		}
	} else {
		for i := 0; i < len(lines); i++ {
			if strings.Contains(lines[i], target) {
				anchorLine = i
			}
		}
	}
	if anchorLine == -1 {
		return existingContent, false
	}

	// Determine indentation from anchor line
	ln := lines[anchorLine]
	j := 0
	for j < len(ln) && (ln[j] == ' ' || ln[j] == '\t') {
		j++
	}
	indent := ln[:j]

	// Normalize snippet newlines
	sn := strings.ReplaceAll(strings.ReplaceAll(snippet, "\r\n", "\n"), "\r", "\n")
	snLines := strings.Split(sn, "\n")
	// Trim trailing empty lines in snippet
	for len(snLines) > 0 && strings.TrimSpace(snLines[len(snLines)-1]) == "" {
		snLines = snLines[:len(snLines)-1]
	}
	if len(snLines) == 0 {
		return existingContent, false
	}

	// Apply indentation to each snippet line if it doesn't already start with whitespace
	var toInsert []string
	for _, sl := range snLines {
		if strings.TrimSpace(sl) == "" {
			toInsert = append(toInsert, sl)
		} else if len(sl) > 0 && (sl[0] == ' ' || sl[0] == '\t') {
			toInsert = append(toInsert, sl)
		} else {
			toInsert = append(toInsert, indent+sl)
		}
	}

	// Choose insertion position
	insertAt := anchorLine
	if behaviour == "insertafterline" {
		insertAt = anchorLine + 1
	}
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}

	// Avoid duplicate immediate insertion (exact block already present at position)
	sameBlock := func(start int) bool {
		if start < 0 || start+len(toInsert) > len(lines) {
			return false
		}
		for i := 0; i < len(toInsert); i++ {
			if strings.TrimRight(lines[start+i], " \t") != strings.TrimRight(toInsert[i], " \t") {
				return false
			}
		}
		return true
	}
	if sameBlock(insertAt) {
		return existingContent, false
	}

	// Insert
	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, toInsert...)
	newLines = append(newLines, lines[insertAt:]...)
	return strings.Join(newLines, "\n"), true
}

// insertSnippetBelowMarker inserts snippet on a new line immediately below the
// ADD marker line for the given key. Occurrence can be "first" or anything else
// (treated as "last"). Indentation of the marker line is applied to snippet lines.
func insertSnippetBelowMarker(existingContent, key, snippet, occurrence string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || strings.TrimSpace(snippet) == "" {
		return existingContent, false
	}
	// Locate marker lines for this key (ABOVE or BELOW)
	pattern := regexp.MustCompile(`(?m)^[ \t]*//\s*ADD\s+` + regexp.QuoteMeta(key) + `\s+(?:BELOW|ABOVE)\s*$`)
	lines := strings.Split(existingContent, "\n")
	var matches []int
	for i := 0; i < len(lines); i++ {
		if pattern.MatchString(lines[i]) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return existingContent, false
	}
	anchorLine := matches[len(matches)-1]
	if strings.ToLower(strings.TrimSpace(occurrence)) == "first" {
		anchorLine = matches[0]
	}

	// Determine indentation from anchor line
	ln := lines[anchorLine]
	j := 0
	for j < len(ln) && (ln[j] == ' ' || ln[j] == '\t') {
		j++
	}
	indent := ln[:j]

	// Normalize snippet newlines
	sn := strings.ReplaceAll(strings.ReplaceAll(snippet, "\r\n", "\n"), "\r", "\n")
	snLines := strings.Split(sn, "\n")
	for len(snLines) > 0 && strings.TrimSpace(snLines[len(snLines)-1]) == "" {
		snLines = snLines[:len(snLines)-1]
	}
	if len(snLines) == 0 {
		return existingContent, false
	}

	// Apply indentation
	var toInsert []string
	for _, sl := range snLines {
		if strings.TrimSpace(sl) == "" {
			toInsert = append(toInsert, sl)
		} else if len(sl) > 0 && (sl[0] == ' ' || sl[0] == '\t') {
			toInsert = append(toInsert, sl)
		} else {
			toInsert = append(toInsert, indent+sl)
		}
	}

	insertAt := anchorLine + 1
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}

	// Insert without adjacent-block deduplication
	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, toInsert...)
	newLines = append(newLines, lines[insertAt:]...)
	return strings.Join(newLines, "\n"), true
}

// insertSnippetAboveMarker inserts snippet on a new line immediately above the
// ADD marker line for the given key. Occurrence can be "first" or anything else
// (treated as "last"). Indentation of the marker line is applied to snippet lines.
func insertSnippetAboveMarker(existingContent, key, snippet, occurrence string) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" || strings.TrimSpace(snippet) == "" {
		return existingContent, false
	}
	// Locate marker lines for this key (ABOVE or BELOW)
	pattern := regexp.MustCompile(`(?m)^[ \t]*//\s*ADD\s+` + regexp.QuoteMeta(key) + `\s+(?:BELOW|ABOVE)\s*$`)
	lines := strings.Split(existingContent, "\n")
	var matches []int
	for i := 0; i < len(lines); i++ {
		if pattern.MatchString(lines[i]) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return existingContent, false
	}
	anchorLine := matches[len(matches)-1]
	if strings.ToLower(strings.TrimSpace(occurrence)) == "first" {
		anchorLine = matches[0]
	}

	// Determine indentation from anchor line
	ln := lines[anchorLine]
	j := 0
	for j < len(ln) && (ln[j] == ' ' || ln[j] == '\t') {
		j++
	}
	indent := ln[:j]

	// Normalize snippet newlines
	sn := strings.ReplaceAll(strings.ReplaceAll(snippet, "\r\n", "\n"), "\r", "\n")
	snLines := strings.Split(sn, "\n")
	for len(snLines) > 0 && strings.TrimSpace(snLines[len(snLines)-1]) == "" {
		snLines = snLines[:len(snLines)-1]
	}
	if len(snLines) == 0 {
		return existingContent, false
	}

	// Apply indentation
	var toInsert []string
	for _, sl := range snLines {
		if strings.TrimSpace(sl) == "" {
			toInsert = append(toInsert, sl)
		} else if len(sl) > 0 && (sl[0] == ' ' || sl[0] == '\t') {
			toInsert = append(toInsert, sl)
		} else {
			toInsert = append(toInsert, indent+sl)
		}
	}

	insertAt := anchorLine
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}

	// Avoid duplicating the exact block immediately above the marker
	sameBlock := func(end int) bool {
		start := end - len(toInsert)
		if start < 0 || end > len(lines) {
			return false
		}
		for i := 0; i < len(toInsert); i++ {
			if strings.TrimRight(lines[start+i], " \t") != strings.TrimRight(toInsert[i], " \t") {
				return false
			}
		}
		return true
	}
	if sameBlock(insertAt) {
		return existingContent, false
	}

	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, toInsert...)
	newLines = append(newLines, lines[insertAt:]...)
	return strings.Join(newLines, "\n"), true
}

// insertMarkerAndSnippetAtTarget inserts a marker line immediately before the
// inserted snippet block relative to the target line. The marker is always of
// the form "// ADD <key> BELOW" so future merges insert below the marker.
// behaviour controls whether the block is placed before or after the target line
// (insertbeforeline | insertafterline). Occurrence can be "first" or anything else
// (treated as "last").
func insertMarkerAndSnippetAtTarget(existingContent, key, snippet, target, behaviour, occurrence string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || strings.TrimSpace(snippet) == "" {
		return existingContent, false
	}
	behaviour = strings.ToLower(strings.TrimSpace(behaviour))
	if behaviour == "insertnextline" {
		behaviour = "insertafterline"
	}
	if behaviour != "insertbeforeline" && behaviour != "insertafterline" {
		behaviour = "insertafterline"
	}
	lines := strings.Split(existingContent, "\n")
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	anchorLine := -1
	if occ == "first" {
		for i := 0; i < len(lines); i++ {
			if strings.Contains(lines[i], target) {
				anchorLine = i
				break
			}
		}
	} else {
		for i := 0; i < len(lines); i++ {
			if strings.Contains(lines[i], target) {
				anchorLine = i
			}
		}
	}
	if anchorLine == -1 {
		return existingContent, false
	}

	// Determine indentation from anchor line
	ln := lines[anchorLine]
	j := 0
	for j < len(ln) && (ln[j] == ' ' || ln[j] == '\t') {
		j++
	}
	indent := ln[:j]

	// Normalize snippet newlines
	sn := strings.ReplaceAll(strings.ReplaceAll(snippet, "\r\n", "\n"), "\r", "\n")
	snLines := strings.Split(sn, "\n")
	for len(snLines) > 0 && strings.TrimSpace(snLines[len(snLines)-1]) == "" {
		snLines = snLines[:len(snLines)-1]
	}
	if len(snLines) == 0 {
		return existingContent, false
	}

	// Apply indentation
	var toInsert []string
	marker := indent + "// ADD " + key + " BELOW"
	toInsert = append(toInsert, marker)
	for _, sl := range snLines {
		if strings.TrimSpace(sl) == "" {
			toInsert = append(toInsert, sl)
		} else if len(sl) > 0 && (sl[0] == ' ' || sl[0] == '\t') {
			toInsert = append(toInsert, sl)
		} else {
			toInsert = append(toInsert, indent+sl)
		}
	}

	// Choose insertion position (marker goes immediately before snippet)
	insertAt := anchorLine
	if behaviour == "insertafterline" {
		insertAt = anchorLine + 1
	}
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(lines) {
		insertAt = len(lines)
	}

	// Avoid duplicate immediate insertion
	sameBlock := func(start int) bool {
		if start < 0 || start+len(toInsert) > len(lines) {
			return false
		}
		for i := 0; i < len(toInsert); i++ {
			if strings.TrimRight(lines[start+i], " \t") != strings.TrimRight(toInsert[i], " \t") {
				return false
			}
		}
		return true
	}
	if sameBlock(insertAt) {
		return existingContent, false
	}

	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, toInsert...)
	newLines = append(newLines, lines[insertAt:]...)
	return strings.Join(newLines, "\n"), true
}

// conditionalReplace performs a one-time replacement of target with replacement
// if requireAbsent is not present in the content.
func conditionalReplace(existingContent, target, requireAbsent, replacement, occurrence string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" || replacement == "" {
		return existingContent, false
	}
	if strings.TrimSpace(requireAbsent) != "" && strings.Contains(existingContent, requireAbsent) {
		return existingContent, false
	}
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	if occ == "first" {
		if idx := strings.Index(existingContent, target); idx >= 0 {
			return existingContent[:idx] + replacement + existingContent[idx+len(target):], true
		}
		return existingContent, false
	}
	if idx := strings.LastIndex(existingContent, target); idx >= 0 {
		return existingContent[:idx] + replacement + existingContent[idx+len(target):], true
	}
	return existingContent, false
}

// replaceBetweenAnchors replaces content between start and end anchors (inclusive)
// with the given replacement. It respects occurrence (first|last) for selecting
// which anchored region to replace. If requireAbsent is non-empty and present in
// existingContent, no replacement is performed.
func replaceBetweenAnchors(existingContent, start, end, requireAbsent, replacement, occurrence string) (string, bool) {
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" || replacement == "" {
		return existingContent, false
	}
	if strings.TrimSpace(requireAbsent) != "" && strings.Contains(existingContent, requireAbsent) {
		return existingContent, false
	}

	// Find occurrence of start anchor
	var startIdx int
	var found bool
	occ := strings.ToLower(strings.TrimSpace(occurrence))
	if occ == "first" {
		if i := strings.Index(existingContent, start); i >= 0 {
			startIdx = i
			found = true
		}
	} else {
		if i := strings.LastIndex(existingContent, start); i >= 0 {
			startIdx = i
			found = true
		}
	}
	if !found {
		return existingContent, false
	}

	// Find end anchor after start
	afterStart := existingContent[startIdx+len(start):]
	endRel := strings.Index(afterStart, end)
	if endRel < 0 {
		return existingContent, false
	}
	endIdx := startIdx + len(start) + endRel + len(end)

	// Replace from startIdx to endIdx with replacement
	return existingContent[:startIdx] + replacement + existingContent[endIdx:], true
}

// removeSnippetMarkers removes the marker lines (START/END) from the content.
func removeSnippetMarkers(content string) string {
	var output []string
	lines := strings.Split(content, "\n")
	collecting := false
	for _, line := range lines {
		if startMarkerRegex.MatchString(line) {
			collecting = true
			continue
		}
		if collecting && endMarkerRegex.MatchString(line) {
			collecting = false
			continue
		}
		output = append(output, line)
	}
	return strings.Join(output, "\n")
}

// extractSnippets scans the content for snippet groups.
// Returns map key -> snippet code.
func extractSnippets(content string) (map[string]string, error) {
	snippets := make(map[string][]string)
	lines := strings.Split(content, "\n")
	var currentKey string
	collecting := false
	for _, line := range lines {
		if m := startMarkerRegex.FindStringSubmatch(line); m != nil {
			currentKey = strings.TrimSpace(m[1])
			collecting = true
			snippets[currentKey] = []string{}
			continue
		}
		if collecting {
			if m := endMarkerRegex.FindStringSubmatch(line); m != nil {
				collecting = false
				currentKey = ""
				continue
			}
			snippets[currentKey] = append(snippets[currentKey], line)
		}
	}
	result := make(map[string]string)
	for key, lines := range snippets {
		result[key] = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return result, nil
}

// augmentTemplateWithFallbackSnippets adds snippet groups from fallback markers so marker-less
// fallback insertions have their code available during smartMerge.
func augmentTemplateWithFallbackSnippets(templateContent string, actions []InsertionAction, placeholders map[string]string) string {
	if len(actions) == 0 {
		return templateContent
	}
	snippetMap, _ := extractSnippets(templateContent)
	for _, m := range actions {
		nm := m.normalized()
		mk := strings.TrimSpace(nm.Title)
		if mk == "" {
			continue
		}
		if _, exists := snippetMap[mk]; exists {
			continue
		}
		if nm.Logic.Spec != nil && strings.TrimSpace(nm.Logic.Spec.Content) != "" {
			snippetMap[mk] = replacePlaceholders(nm.Logic.Spec.Content, placeholders)
		}
	}
	if len(snippetMap) == 0 {
		return templateContent
	}
	var b strings.Builder
	b.WriteString(templateContent)
	if !strings.HasSuffix(templateContent, "\n") {
		b.WriteString("\n")
	}
	for k, sn := range snippetMap {
		b.WriteString("// START OF ")
		b.WriteString(k)
		b.WriteString("\n")
		b.WriteString(sn)
		b.WriteString("\n")
		b.WriteString("// END OF ")
		b.WriteString(k)
		b.WriteString("\n")
	}
	return b.String()
}

// canonicalizeSlugAliases normalizes alias forms so merges treat
// `slug.current as slug` and `"slug": slug.current` as equivalent.
func canonicalizeSlugAliases(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	re := regexp.MustCompile(`slug\.current\s+as\s+slug`)
	return re.ReplaceAllString(s, `"slug": slug.current`)
}

// smartMerge merges template content into an existing file based on markers.
func smartMerge(existingContent, templateContent string) (string, error) {
	snippetMap, _ := extractSnippets(templateContent)

	// Helper that searches snippetMap by exact or fuzzy key
	findSnippetForKey := func(key string) (string, bool) {
		if sn, ok := snippetMap[key]; ok && strings.TrimSpace(sn) != "" {
			return sn, true
		}
		return findSnippetForKeyGlobal(snippetMap, key)
	}

	lines := strings.Split(existingContent, "\n")
	normalizeForContains := func(s string) string {
		parts := strings.Split(s, "\n")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return strings.Join(parts, "\n")
	}
	normalizeNoSpaces := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\t", ""), " ", ""), "\r", "")
	}
	existingNormalized := normalizeForContains(existingContent)
	existingNormalized = canonicalizeSlugAliases(existingNormalized)
	existingLineSetNoSpaces := map[string]bool{}
	for _, ln := range lines {
		existingLineSetNoSpaces[normalizeNoSpaces(ln)] = true
		// cache canonicalized variant for slug alias lines
		if strings.Contains(strings.ToLower(ln), "slug.current as slug") {
			canonical := strings.ReplaceAll(ln, "slug.current as slug", "\"slug\": slug.current")
			existingLineSetNoSpaces[normalizeNoSpaces(canonical)] = true
			trimmed := strings.TrimSpace(canonical)
			if !strings.HasSuffix(trimmed, ",") {
				existingLineSetNoSpaces[normalizeNoSpaces(canonical+",")] = true
			}
		}
	}

	var mergedLines []string
	insertedForKey := map[string]bool{}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if matches := addMarkerRegex.FindStringSubmatch(line); matches != nil {
			key := strings.TrimSpace(matches[1])
			position := strings.ToUpper(matches[2])
			if insertedForKey[key] {
				continue
			}
			if snippet, ok := findSnippetForKey(key); ok && snippet != "" {
				snippetLines := strings.Split(snippet, "\n")
				snippetNormalized := normalizeForContains(snippet)
				snippetNormalized = canonicalizeSlugAliases(snippetNormalized)
				alreadyPresent := strings.Contains(existingNormalized, snippetNormalized)
				if !alreadyPresent && len(snippetLines) > 0 {
					first := strings.TrimSpace(snippetLines[0])
					firstNoSpaces := normalizeNoSpaces(first)
					if strings.HasSuffix(first, ",") && existingLineSetNoSpaces[firstNoSpaces] {
						alreadyPresent = true
					}
					if !alreadyPresent && (strings.HasPrefix(first, "import ") || strings.Contains(first, "require(")) {
						modPath := ""
						if idx := strings.Index(first, " from "); idx != -1 {
							q := first[idx+6:]
							q = strings.TrimSpace(q)
							if len(q) > 0 && (q[0] == '\'' || q[0] == '"') {
								end := strings.LastIndex(q, string(q[0]))
								if end > 0 {
									modPath = q[1:end]
								}
							}
						} else if strings.Contains(first, "require(") {
							start := strings.Index(first, "require(")
							if start >= 0 {
								rest := first[start+8:]
								rest = strings.TrimSpace(rest)
								if len(rest) > 0 && (rest[0] == '\'' || rest[0] == '"') {
									end := strings.Index(rest[1:], string(rest[0]))
									if end >= 0 {
										modPath = rest[1 : 1+end]
									}
								}
							}
						}
						if modPath != "" {
							impRegex := regexp.MustCompile(`(?m)^\s*(import\s+.*from\s+['"]` + regexp.QuoteMeta(modPath) + `['"]|.*require\(['"]` + regexp.QuoteMeta(modPath) + `['"]\))`)
							if impRegex.FindStringIndex(existingContent) != nil {
								alreadyPresent = true
							}
						}
					}
				}
				if position == "BELOW" {
					mergedLines = append(mergedLines, line)
					if alreadyPresent {
						insertedForKey[key] = true
						continue
					}
					if i+1+len(snippetLines) <= len(lines) {
						window := normalizeForContains(strings.Join(lines[i+1:i+1+len(snippetLines)], "\n"))
						if window == snippetNormalized {
							insertedForKey[key] = true
							continue
						}
					}
					for _, s := range snippetLines {
						mergedLines = append(mergedLines, s)
					}
					insertedForKey[key] = true
					continue
				} else if position == "ABOVE" {
					if alreadyPresent {
						mergedLines = append(mergedLines, line)
						insertedForKey[key] = true
						continue
					}
					if len(mergedLines) >= len(snippetLines) {
						window := normalizeForContains(strings.Join(mergedLines[len(mergedLines)-len(snippetLines):], "\n"))
						if window == snippetNormalized {
							mergedLines = append(mergedLines, line)
							insertedForKey[key] = true
							continue
						}
					}
					for _, s := range snippetLines {
						mergedLines = append(mergedLines, s)
					}
					mergedLines = append(mergedLines, line)
					insertedForKey[key] = true
					continue
				}
			}
		}
		mergedLines = append(mergedLines, line)
	}
	return strings.Join(mergedLines, "\n"), nil
}

// cleanupIndexerContent removes duplicate import statements and duplicate array items in schemaTypes.
func cleanupIndexerContent(content string) string {
	lines := strings.Split(content, "\n")
	importFromRegex := regexp.MustCompile(`^\s*import\s+.*from\s+['"]([^'\"]+)['"]`)
	requireRegex := regexp.MustCompile(`^\s*(?:const|let|var)\s+\w+\s*=\s*require\(['"]([^'\"]+)['"]\)`) // basic CJS
	arrayItemRegex := regexp.MustCompile(`^\s*[_$a-zA-Z][_$a-zA-Z0-9]*\s*,\s*$`)

	seenImport := map[string]bool{}

	inSchemaTypes := false
	bracketDepth := 0
	seenArrayItem := map[string]bool{}
	schemaTypesStart := regexp.MustCompile(`^\s*export\s+const\s+schemaTypes\s*=\s*\[`)

	var out []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if !inSchemaTypes && schemaTypesStart.MatchString(ln) {
			inSchemaTypes = true
			bracketDepth = 1
			seenArrayItem = map[string]bool{}
			out = append(out, ln)
			continue
		}

		if inSchemaTypes {
			for i := 0; i < len(ln); i++ {
				if ln[i] == '[' {
					bracketDepth++
				} else if ln[i] == ']' {
					bracketDepth--
				}
			}
			if arrayItemRegex.MatchString(ln) {
				key := strings.ReplaceAll(strings.ReplaceAll(trim, "\t", ""), " ", "")
				if seenArrayItem[key] {
					// skip duplicate
				} else {
					seenArrayItem[key] = true
					out = append(out, ln)
				}
			} else {
				out = append(out, ln)
			}
			if bracketDepth <= 0 {
				inSchemaTypes = false
			}
			continue
		}

		if trim == "" {
			out = append(out, ln)
			continue
		}
		if startMarkerRegex.MatchString(ln) || endMarkerRegex.MatchString(ln) || addMarkerRegex.MatchString(ln) {
			out = append(out, ln)
			continue
		}
		if m := importFromRegex.FindStringSubmatch(ln); m != nil {
			mod := m[1]
			if seenImport[mod] {
				continue
			}
			seenImport[mod] = true
			out = append(out, ln)
			continue
		}
		if m := requireRegex.FindStringSubmatch(ln); m != nil {
			mod := m[1]
			if seenImport[mod] {
				continue
			}
			seenImport[mod] = true
			out = append(out, ln)
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// ensureExportForLinkReference ensures "const linkReference|linkFields" becomes exported.
func ensureExportForLinkReference(content string) string {
	re := regexp.MustCompile(`(?m)^(\s*)const\s+(linkReference|linkFields)\b`)
	return re.ReplaceAllString(content, "$1export const $2")
}
