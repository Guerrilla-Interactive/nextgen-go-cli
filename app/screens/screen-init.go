package screens

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Guerrilla-Interactive/nextgen-go-cli/app"
	tea "github.com/charmbracelet/bubbletea"
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
