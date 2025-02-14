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

// SummarizeLimitedProjectStats returns a formatted string of project stats limited to 'limit' items.
func SummarizeLimitedProjectStats(recognizedPkgs []string, limit int) string {
	if len(recognizedPkgs) == 0 {
		return ""
	}
	pkgList := GroupRecognizedPackages(recognizedPkgs)
	if len(pkgList) > limit {
		pkgList = pkgList[:limit]
	}
	return RenderPackagesHorizontally(pkgList, 6)
}

// SummarizeFullProjectStats returns a formatted string of all recognized packages.
func SummarizeFullProjectStats(recognizedPkgs []string) string {
	if len(recognizedPkgs) == 0 {
		return ""
	}
	pkgList := GroupRecognizedPackages(recognizedPkgs)
	return RenderPackagesHorizontally(pkgList, 6)
}

// SummarizeProjectStats (for the main screen) returns a formatted project stats summary
// limited to a maximum of 3 items.
func SummarizeProjectStats(recognizedPkgs []string) string {
	return SummarizeLimitedProjectStats(recognizedPkgs, 3)
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

	// Define known CMS frameworks using normalized keys.
	cmsFrameworks := map[string]bool{
		"wordpress": true,
		"drupal":    true,
		"joomla":    true,
		"shopify":   true,
		"magento":   true,
		"sanity":    true,
	}

	var otherPkgs []string
	var reactCandidate string
	cssCount := 0
	var cssCandidate string
	cmsCount := 0
	var cmsCandidate string

	// For non-group packages, avoid duplicates.
	seen := map[string]bool{}

	for _, pkg := range pkgs {
		norm := normalizePkgName(pkg)
		// Check VIP groups first.
		if reactFrameworks[norm] {
			if reactCandidate == "" {
				reactCandidate = pkg
			} else if norm == "nextjs" { // prefer nextjs
				reactCandidate = pkg
			}
			continue
		}
		if norm == "react" {
			if reactCandidate == "" {
				reactCandidate = pkg
			}
			continue
		}
		if cssFrameworks[norm] {
			cssCount++
			if cssCandidate == "" {
				cssCandidate = pkg
			}
			continue
		}
		if cmsFrameworks[norm] {
			cmsCount++
			if cmsCandidate == "" {
				cmsCandidate = pkg
			}
			continue
		}

		// For all other packages, add if not already added.
		if !seen[pkg] {
			otherPkgs = append(otherPkgs, pkg)
			seen[pkg] = true
		}
	}

	// Build VIP packages list.
	var vip []string
	if reactCandidate != "" {
		vip = append(vip, reactCandidate)
	}
	if cmsCandidate != "" {
		if cmsCount > 1 {
			vip = append(vip, fmt.Sprintf("%d CMS Packages", cmsCount))
		} else {
			vip = append(vip, cmsCandidate)
		}
	}
	if cssCount > 0 {
		if cssCount > 1 {
			vip = append(vip, fmt.Sprintf("%d CSS Packages", cssCount))
		} else {
			vip = append(vip, cssCandidate)
		}
	}

	// Prepend VIP packages before the other (non-VIP) packages.
	finalGroup := append(vip, otherPkgs...)
	return finalGroup
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
