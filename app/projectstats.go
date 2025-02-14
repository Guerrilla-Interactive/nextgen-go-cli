package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// nonAlphaNumRegex matches any sequence of characters that is not a lowercase letter or digit.
var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// normalizePkgName returns a normalized version of the package name by
// converting to lower case and removing all non-alphanumeric characters.
func normalizePkgName(s string) string {
	return nonAlphaNumRegex.ReplaceAllString(strings.ToLower(s), "")
}

// SummarizeProjectStats returns a formatted string of project stats.
// It groups recognized packages (e.g. React frameworks are deduplicated and multiple CSS
// frameworks are summarized) and renders them in a grid with up to maxCols columns.
func SummarizeProjectStats(recognizedPkgs []string) string {
	if len(recognizedPkgs) == 0 {
		return ""
	}
	groupedPkgs := GroupRecognizedPackages(recognizedPkgs)
	return RenderPackagesHorizontally(groupedPkgs, 6)
}

// GroupRecognizedPackages processes a list of package names, grouping React-based frameworks
// and CSS frameworks. For example:
//   - If "Next.js" (or Gatsby, etc.) is detected, only that candidate is kept (with a preference for Next.js).
//   - If multiple CSS frameworks are detected, they are summarized as "N CSS Packages".
func GroupRecognizedPackages(pkgs []string) []string {
	// Define known react frameworks using normalized keys.
	reactFrameworks := map[string]bool{
		"nextjs":      true,
		"gatsby":      true,
		"reactnative": true,
		"remix":       true,
		"blitzjs":     true,
	}
	// Define known CSS frameworks using normalized keys.
	cssFrameworks := map[string]bool{
		"tailwindcss":      true,
		"bootstrap":        true,
		"bulma":            true,
		"foundation":       true,
		"semanticui":       true,
		"materialui":       true,
		"chakraui":         true,
		"antdesign":        true,
		"styledcomponents": true,
	}

	cmsFrameworks := map[string]bool{
		"wordpress": true,
		"drupal":    true,
		"joomla":    true,
		"shopify":   true,
		"magento":   true,
		"sanity":    true,
	}

	var finalPkgs []string
	var reactCandidate string
	cssCount := 0
	var cssCandidate string
	cmsCount := 0
	var cmsCandidate string

	// For non-group packages, avoid duplicates.
	seen := map[string]bool{}

	for _, pkg := range pkgs {
		norm := normalizePkgName(pkg)
		// If package is in the React frameworks group.
		if reactFrameworks[norm] {
			if reactCandidate == "" {
				reactCandidate = pkg
			} else {
				// Give preference to "nextjs" if encountered.
				if norm == "nextjs" {
					reactCandidate = pkg
				}
			}
			continue
		}
		// For the base "react" itself, only consider it if no framework candidate was already found.
		if norm == "react" {
			if reactCandidate == "" {
				reactCandidate = pkg
			}
			continue
		}
		// If package is in the CSS group.
		if cssFrameworks[norm] {
			cssCount++
			if cssCandidate == "" {
				cssCandidate = pkg
			}
			continue
		}

		// If package is in the CMS group.
		if cmsFrameworks[norm] {
			cmsCount++
			if cmsCandidate == "" {
				cmsCandidate = pkg
			}
			continue
		}

		// For all other packages, add if not already added.
		if !seen[pkg] {
			finalPkgs = append(finalPkgs, pkg)
			seen[pkg] = true
		}
	}

	// Append the React candidate (if any) only once.
	if reactCandidate != "" {
		finalPkgs = append(finalPkgs, reactCandidate)
	}

	// Append CSS frameworks – if more than one CSS framework was detected, summarize the count.
	if cssCount > 0 {
		if cssCount == 1 {
			finalPkgs = append(finalPkgs, cssCandidate)
		} else {
			finalPkgs = append(finalPkgs, fmt.Sprintf("%d CSS Packages", cssCount))
		}
	}

	// Append CMS frameworks – if more than one CMS framework was detected, summarize the count.

	return finalPkgs
}

// RenderPackagesHorizontally displays items in a grid of up to maxCols columns
// with a right margin for spacing.
func RenderPackagesHorizontally(items []string, maxCols int) string {
	if len(items) == 0 {
		return ""
	}

	// Determine the number of columns.
	cols := maxCols
	if len(items) < cols {
		cols = len(items)
	}
	// Compute the number of rows (ceiling division).
	rows := (len(items) + cols - 1) / cols

	// Define a column style: left aligned with a right margin and gray color.
	colStyle := lipgloss.NewStyle().
		MarginRight(2).
		Align(lipgloss.Left).
		Foreground(lipgloss.Color("#888"))

	var lines []string
	for r := 0; r < rows; r++ {
		var line string
		for c := 0; c < cols; c++ {
			index := c*rows + r
			if index >= len(items) {
				break
			}
			// Add a bullet separator for items after the first in a row.
			if c > 0 {
				line += lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("•  ")
			}
			line += colStyle.Render(items[index])
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}
