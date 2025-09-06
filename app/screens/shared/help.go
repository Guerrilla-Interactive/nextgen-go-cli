package shared

import (
    "strings"
    "github.com/Guerrilla-Interactive/nextgen-go-cli/app"
)

// Footer joins navigation tips with a consistent separator and applies
// the global help style for footers.
func Footer(parts ...string) string {
    if len(parts) == 0 {
        return ""
    }
    text := strings.Join(parts, "  •  ")
    return app.HelpStyle.Render(text)
}

