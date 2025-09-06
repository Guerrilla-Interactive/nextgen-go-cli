package shared

import (
    "fmt"
    "path/filepath"
    "github.com/charmbracelet/lipgloss"
)

// ComputeLeftPanelWidth returns a stable left column width based on the
// terminal width with sensible clamping and ensuring room for the right side.
//
// Rules:
// - Target ~45% of terminal width
// - Clamp to [minLeft, maxLeft]
// - Reserve a single-space gap and at least rightMin for the right panel
func ComputeLeftPanelWidth(termWidth int) int {
    const (
        defaultLeft = 56
        minLeft     = 36
        maxLeft     = 72
        gap         = 1
        rightMin    = 32
    )
    if termWidth <= 0 {
        return defaultLeft
    }
    left := (termWidth * 9) / 20 // ~45%
    if left < minLeft {
        left = minLeft
    }
    if left > maxLeft {
        left = maxLeft
    }
    // Ensure the right panel has at least rightMin (plus gap)
    if left+gap+rightMin > termWidth {
        left = termWidth - gap - rightMin
    }
    if left < 20 { // last-ditch lower bound for very small terminals
        left = 20
    }
    return left
}

// ComputeRightPanelWidth returns the remaining width after the left panel and a gap.
func ComputeRightPanelWidth(termWidth, left, gap int) int {
    w := termWidth - left - gap
    if w < 0 {
        w = 0
    }
    return w
}

// ComputeLeftPanelWidthFavorLeft returns a wider left column than the default
// by targeting ~60% of terminal width, with broader clamps and no hard
// reservation for the right panel (allowing the right to overflow if needed).
func ComputeLeftPanelWidthFavorLeft(termWidth int) int {
    const (
        defaultLeft = 64
        minLeft     = 44
        maxLeft     = 92
        gap         = 1
        rightMin    = 0 // allow right to overflow; no reservation
    )
    if termWidth <= 0 {
        return defaultLeft
    }
    left := (termWidth * 3) / 5 // ~60%
    if left < minLeft {
        left = minLeft
    }
    if left > maxLeft {
        left = maxLeft
    }
    if left+gap+rightMin > termWidth {
        left = termWidth - gap - rightMin
    }
    if left < 20 {
        left = 20
    }
    return left
}

// ProjectHeader renders a standard gray header with the current project folder name.
// Keeps the emoji consistent across screens.
func ProjectHeader(projectPath string) string {
    folderName := filepath.Base(projectPath)
    return lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render(fmt.Sprintf("📦 %s", folderName))
}
