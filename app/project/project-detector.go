package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ProjectInfo stores information about a detected project
type ProjectInfo struct {
	RootPath         string                       // Absolute path to project root
	Name             string                       // Project name
	Type             string                       // Primary detected project type (e.g., nextjs, react, vue, generic-npm, git)
	DetectedPackages []string                     // List of all detected packages/frameworks based on dependencies
	PackageInfo      map[string]string            // Selected info from package.json (name, version, description)
	Dependencies     map[string]string            // Map of dependencies from package.json
	DevDependencies  map[string]string            // Map of devDependencies from package.json
	GitInfo          map[string]string            // Info from .git config (if available)
	UsageCount       int                          // Times NextGen was used in this project
	LastAccessTime   int64                        // Last time project was accessed (Unix timestamp)
	Environments     map[string]EnvironmentConfig // Environment configurations (Placeholder for now)
}

// EnvironmentConfig placeholder (will be defined fully later)
type EnvironmentConfig struct {
	Name      string
	Variables map[string]string
}

// nonAlphaNumRegex matches any sequence of characters that is not a lowercase letter or digit.
// Used for normalizing package names for easier matching.
var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// normalizePkgName returns a normalized version of the package name.
func normalizePkgName(s string) string {
	return nonAlphaNumRegex.ReplaceAllString(strings.ToLower(s), "")
}

// Define known framework/package names and their normalized versions for detection
var knownPackages = map[string]string{
	"next":               "nextjs",
	"react":              "react",
	"gatsby":             "gatsby",
	"react-native":       "react-native",
	"remix":              "remix",
	"blitz":              "blitzjs", // Blitz.js uses 'blitz' package name
	"vue":                "vue",
	"@angular/core":      "angular",
	"svelte":             "svelte",
	"tailwindcss":        "tailwindcss",
	"bootstrap":          "bootstrap",
	"bulma":              "bulma",
	"foundation-sites":   "foundation",  // Foundation uses 'foundation-sites'
	"semantic-ui-react":  "semantic-ui", // Or 'semantic-ui-css'
	"semantic-ui-css":    "semantic-ui",
	"@mui/material":      "material-ui", // Material UI new package name
	"@material-ui/core":  "material-ui", // Material UI old package name
	"@chakra-ui/react":   "chakra-ui",
	"antd":               "ant-design",
	"styled-components":  "styled-components",
	"emotion":            "emotion", // Often used with styled-components or MUI
	"@wordpress/scripts": "wordpress",
	"drupal":             "drupal", // Less likely in package.json, but check
	"joomla":             "joomla", // Less likely
	"@shopify/cli":       "shopify",
	"@sanity/cli":        "sanity",
	// Add other relevant packages here
}

// DetectProject examines the given directory and parents to find project markers
// It walks up the directory tree looking for common project identifiers.
func DetectProject(startPath string) (ProjectInfo, bool) {
	// Start with the given path and walk up the directory tree
	currentPath := startPath
	for {
		// Check for package.json first (most common indicator)
		if hasPackageJSON, pkgData := checkForPackageJSON(currentPath); hasPackageJSON {
			info, ok := createProjectInfo(currentPath, pkgData, nil, "npm") // Initial type 'npm'
			if ok {
				// Check for Git info in the same directory
				if hasGit, gitData := checkForGit(currentPath); hasGit {
					info.GitInfo = gitData // Add Git info if found
				}
				return info, true
			}
		}

		// Check for .git directory if no package.json was found at this level
		if hasGit, gitData := checkForGit(currentPath); hasGit {
			info, ok := createProjectInfo(currentPath, nil, gitData, "git") // Type 'git'
			if ok {
				return info, true
			}
		}

		// Add checks for other project types here (e.g., go.mod, pyproject.toml)
		// if checkGoProject(currentPath) { ... }

		// Move up one directory
		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			// We've reached the root without finding a project
			break
		}
		currentPath = parentPath
	}

	// No project found
	return ProjectInfo{}, false
}

// checkForPackageJSON looks for package.json and extracts relevant info as a map
func checkForPackageJSON(dir string) (bool, map[string]interface{}) {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		// File doesn't exist or cannot be read
		return false, nil
	}

	var pkgInfo map[string]interface{}
	if err := json.Unmarshal(data, &pkgInfo); err != nil {
		// File exists but is invalid JSON
		// Treat as found, but without data, maybe log this?
		// Consider returning an error instead or logging
		return true, nil
	}

	// Successfully found and parsed package.json
	return true, pkgInfo
}

// checkForGit looks for .git directory and extracts basic info from config
func checkForGit(dir string) (bool, map[string]string) {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// .git directory doesn't exist
		return false, nil
	}

	gitInfo := make(map[string]string)
	gitInfo["hasGitDirectory"] = "true" // Mark that .git exists

	// Read git config
	configPath := filepath.Join(gitDir, "config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Git directory exists but couldn't read config
		// Log error maybe? fmt.Printf("Warning: Could not read .git/config in %s: %v\n", dir, err)
		return true, gitInfo // Indicate Git presence, but limited config data
	}

	// Basic parsing of git config to find remote origin URL
	lines := strings.Split(string(data), "\n")
	inRemoteOrigin := false
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "[remote \"origin\"]") {
			inRemoteOrigin = true
		} else if inRemoteOrigin && strings.HasPrefix(trimmedLine, "url =") {
			parts := strings.SplitN(trimmedLine, "=", 2)
			if len(parts) == 2 {
				gitInfo["remoteOriginUrl"] = strings.TrimSpace(parts[1])
			}
			// Found the URL, no need to continue parsing this section for this simple check
			break
		} else if inRemoteOrigin && strings.HasPrefix(trimmedLine, "[") {
			// Exited the remote origin section before finding URL
			break
		}
	}

	// Optionally, try to get current branch from .git/HEAD
	headPath := filepath.Join(gitDir, "HEAD")
	headData, headErr := os.ReadFile(headPath)
	if headErr == nil {
		headContent := strings.TrimSpace(string(headData))
		if strings.HasPrefix(headContent, "ref: refs/heads/") {
			gitInfo["currentBranch"] = strings.TrimPrefix(headContent, "ref: refs/heads/")
		} else if len(headContent) == 40 { // Detached HEAD state, likely commit hash
			gitInfo["currentBranch"] = "DETACHED"
			gitInfo["currentCommit"] = headContent
		}
	} // else { fmt.Printf("Warning: Could not read .git/HEAD in %s: %v\n", dir, headErr) }

	// Indicate Git presence, with extracted info if available
	return true, gitInfo
}

// createProjectInfo constructs a ProjectInfo struct from detected data.
// It prioritizes package.json for name and type detection.
func createProjectInfo(rootPath string, pkgData map[string]interface{}, gitData map[string]string, primaryType string) (ProjectInfo, bool) {
	absRootPath, err := filepath.Abs(rootPath)
	if err != nil {
		// Could not determine absolute path, treat as invalid
		return ProjectInfo{}, false
	}

	info := ProjectInfo{
		RootPath:         absRootPath,
		Type:             primaryType, // Initial type (npm or git)
		DetectedPackages: []string{},  // Initialize empty slice
		PackageInfo:      make(map[string]string),
		Dependencies:     make(map[string]string),
		DevDependencies:  make(map[string]string),
		GitInfo:          gitData,                            // Assign Git data if provided
		UsageCount:       0,                                  // Initial usage count
		LastAccessTime:   time.Now().Unix(),                  // Set initial access time
		Environments:     make(map[string]EnvironmentConfig), // Initialize empty map
	}

	// Default project name to the directory name
	projectName := filepath.Base(absRootPath)

	// If package.json data is available, process it
	if pkgData != nil {
		// Get project name from package.json if available
		if name, ok := pkgData["name"].(string); ok && name != "" {
			projectName = name
		}

		// Extract basic package info (version, description)
		for _, key := range []string{"version", "description"} {
			if val, ok := pkgData[key].(string); ok {
				info.PackageInfo[key] = val
			}
		}

		// Process dependencies and devDependencies
		detectedPkgsSet := make(map[string]bool) // Use a set to avoid duplicates
		processDependencies := func(depType string, depsInterface interface{}) {
			if depsMap, ok := depsInterface.(map[string]interface{}); ok {
				for pkgName, version := range depsMap {
					versionStr, _ := version.(string) // Store version string
					if depType == "dependencies" {
						info.Dependencies[pkgName] = versionStr
					} else if depType == "devDependencies" {
						info.DevDependencies[pkgName] = versionStr
					}

					// Check against known packages for framework detection
					if frameworkName, known := knownPackages[pkgName]; known {
						detectedPkgsSet[frameworkName] = true
					}
				}
			}
		}

		processDependencies("dependencies", pkgData["dependencies"])
		processDependencies("devDependencies", pkgData["devDependencies"])

		// Convert the set of detected packages to a slice
		for pkg := range detectedPkgsSet {
			info.DetectedPackages = append(info.DetectedPackages, pkg)
		}

		// Determine the primary project type based on detected packages (prioritize)
		if detectedPkgsSet["nextjs"] {
			info.Type = "nextjs"
		} else if detectedPkgsSet["react"] {
			info.Type = "react"
		} else if detectedPkgsSet["angular"] {
			info.Type = "angular"
		} else if detectedPkgsSet["vue"] {
			info.Type = "vue"
		} else if detectedPkgsSet["svelte"] {
			info.Type = "svelte"
		} else if detectedPkgsSet["wordpress"] {
			info.Type = "wordpress"
		} else if detectedPkgsSet["sanity"] {
			info.Type = "sanity"
		} else if detectedPkgsSet["shopify"] {
			info.Type = "shopify"
		}
		// If primaryType was 'npm' and no specific framework detected, keep it as 'npm'

	} // End of processing pkgData

	info.Name = projectName

	return info, true
}
