package screens

// NOTE: Although this file provides the Init screen functionality,
// it is not used for display because the application now skips
// the intro and directly shows the recent commands screen.

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InitProjectCmd returns a Cmd that loads project info (recognized packages, etc.).
func InitProjectCmd(m app.Model) tea.Cmd {
	return func() tea.Msg {
		wd, _ := os.Getwd()
		recPkgs := detectFrameworks(wd)

		m.ProjectPath = wd
		m.RecognizedPkgs = recPkgs
		return m
	}
}

func detectFrameworks(projectPath string) []string {
	knownPackages := map[string]string{
		"next":              "Next.js",
		"sanity":            "Sanity (CMS)",
		"tailwindcss":       "Tailwind CSS",
		"react-email":       "React Email",
		"styled-components": "styled-components",
		"gatsby":            "Gatsby",
		"contentful":        "Contentful",
		"strapi":            "Strapi",
		"vue":               "Vue.js",
		"react":             "React",
		"angular":           "Angular",
	}

	packageJSONPath := filepath.Join(projectPath, "package.json")
	data, err := ioutil.ReadFile(packageJSONPath)
	if err != nil {
		// No recognized packages
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	foundSet := map[string]bool{}
	for dep := range pkg.Dependencies {
		if friendly, ok := knownPackages[dep]; ok {
			foundSet[friendly] = true
		}
	}
	for dep := range pkg.DevDependencies {
		if friendly, ok := knownPackages[dep]; ok {
			foundSet[friendly] = true
		}
	}

	var results []string
	for k := range foundSet {
		results = append(results, k)
	}
	return results
}

// ViewInitScreen builds the initialization screen and anchors the content to the bottom of the terminal.
func ViewInitScreen(m app.Model) string {
	// Create a title for the init screen.
	titleText := "=== Init Screen ==="
	title := app.TitleStyle.Render(titleText)

	// Render the project path.
	pathLine := app.PathStyle.Render("Project path: " + m.ProjectPath)

	// Render the recognized packages if any.
	var pkgInfo string
	if len(m.RecognizedPkgs) > 0 {
		pkgInfo = "Recognized Packages: " + strings.Join(m.RecognizedPkgs, ", ")
	}

	// Build the body content.
	content := title + "\n" + pathLine
	if pkgInfo != "" {
		content += "\n" + pkgInfo
	}

	// Wrap the content in a base container.
	panel := baseContainer(content)

	// Use a fallback if TerminalHeight is zero.
	// (Ensure your model receives an up-to-date WindowSizeMsg.)
	termHeight := m.TerminalHeight
	if termHeight == 0 {
		termHeight = 24
	}

	// Anchor the panel to the bottom
	anchoredPanel := lipgloss.Place(
		lipgloss.Width(panel), // set width to the panel's width
		termHeight,            // set height to the terminal height (or fallback)
		lipgloss.Left,         // horizontal alignment
		lipgloss.Bottom,       // vertical anchoring to the bottom
		panel,                 // the content to anchor
	)

	return anchoredPanel
}
