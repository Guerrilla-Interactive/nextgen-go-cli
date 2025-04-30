package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// // HistoricCommand stores details ... (Remove this duplicate struct definition)
// type HistoricCommand struct {
// 	Name           string            `json:"name"`
// 	Variables      map[string]string `json:"variables"`
// 	Timestamp      int64             `json:"timestamp"`
// 	GeneratedFiles []string          `json:"generatedFiles"`
// }

// --- Add ClipboardCommandSpec ---
// ClipboardCommandSpec stores details about a saved clipboard command.
type ClipboardCommandSpec struct {
	Name       string `json:"name"`       // User-defined name
	Template   string `json:"template"`   // The actual template content
	IsFavorite bool   `json:"isFavorite"` // Flag for favorites
	Timestamp  int64  `json:"timestamp"`  // When it was saved
}

// ProjectInfo stores information about a detected project
// ... (existing struct definition)

// ProjectRegistry holds information about all known projects
// It uses a mutex for safe concurrent access if needed in the future.
type ProjectRegistry struct {
	Projects                map[string]ProjectInfo          `json:"projects"`
	LastUsedPath            string                          `json:"lastUsedPath"`
	GlobalUsages            int                             `json:"globalUsages"`
	ClipboardCommands       map[string]ClipboardCommandSpec `json:"clipboardCommands"`
	NativeCommands          map[string]string               `json:"nativeCommands"`
	FavoriteNativeCommands  map[string]bool                 `json:"favoriteNativeCommands"`
	FavoriteProjectCommands map[string]bool                 `json:"favoriteProjectCommands"`
	RegistryPath            string                          `json:"-"`
	mu                      sync.RWMutex                    `json:"-"`
}

// registryFileName is the name of the file used to store the project registry.
const registryFileName = "projects.json"

// getRegistryPath determines the appropriate path for the registry file.
// It typically resides in a hidden config directory within the user's home directory.
func getRegistryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".config", "nextgen-cli") // Using .config standard
	if err := os.MkdirAll(configDir, 0750); err != nil {          // Use 0750 for permissions
		return "", fmt.Errorf("could not create config directory %s: %w", configDir, err)
	}
	return filepath.Join(configDir, registryFileName), nil
}

// LoadProjectRegistry loads the project registry from disk.
// If the registry file doesn't exist, it initializes an empty registry.
func LoadProjectRegistry() (*ProjectRegistry, error) {
	registryPath, err := getRegistryPath()
	if err != nil {
		return nil, err // Error determining path
	}

	registry := &ProjectRegistry{
		Projects:                make(map[string]ProjectInfo),
		ClipboardCommands:       make(map[string]ClipboardCommandSpec),
		NativeCommands:          make(map[string]string),
		FavoriteNativeCommands:  make(map[string]bool),
		FavoriteProjectCommands: make(map[string]bool),
		LastUsedPath:            "",
		GlobalUsages:            0,
		RegistryPath:            registryPath,
	}

	// Try to load existing registry data
	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return the initialized empty registry
			// We can save it immediately to ensure the directory/file exists
			if saveErr := registry.Save(); saveErr != nil {
				// Log this warning? fmt.Printf("Warning: Could not save initial empty registry: %v\n", saveErr)
			}
			return registry, nil
		} else {
			// Other error reading the file
			return nil, fmt.Errorf("error reading registry file %s: %w", registryPath, err)
		}
	}

	// File exists, unmarshal the JSON data
	registry.mu.Lock() // Lock before modifying registry data
	defer registry.mu.Unlock()
	if err := json.Unmarshal(data, &registry); err != nil {
		// Handle potential corruption - maybe backup old file and start fresh?
		return nil, fmt.Errorf("error unmarshalling registry file %s: %w. File might be corrupt.", registryPath, err)
	}

	// Ensure nested maps are initialized if loaded from an empty file
	if registry.Projects == nil {
		registry.Projects = make(map[string]ProjectInfo)
	}
	if registry.ClipboardCommands == nil {
		registry.ClipboardCommands = make(map[string]ClipboardCommandSpec)
	}
	if registry.FavoriteNativeCommands == nil {
		registry.FavoriteNativeCommands = make(map[string]bool)
	}
	if registry.FavoriteProjectCommands == nil {
		registry.FavoriteProjectCommands = make(map[string]bool)
	}
	if registry.NativeCommands == nil {
		registry.NativeCommands = make(map[string]string)
	}

	// --- Ensure CommandHistory is initialized for each loaded project ---
	for key, projectInfo := range registry.Projects {
		if projectInfo.CommandHistory == nil {
			// Initialize if nil after unmarshalling
			projectInfo.CommandHistory = []HistoricCommand{}
			registry.Projects[key] = projectInfo // Update the map with the initialized struct
		}
	}

	// We need to re-assign the RegistryPath as it's ignored by json
	registry.RegistryPath = registryPath

	return registry, nil
}

// Save persists the current state of the project registry to disk.
func (r *ProjectRegistry) Save() error {
	r.mu.RLock()                                 // Read lock to marshal data
	data, err := json.MarshalIndent(r, "", "  ") // Use indentation for readability
	r.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("error marshalling registry: %w", err)
	}

	// Write data to the registry file
	if err := os.WriteFile(r.RegistryPath, data, 0640); err != nil { // Use 0640 for permissions
		return fmt.Errorf("error writing registry file %s: %w", r.RegistryPath, err)
	}

	return nil
}

// AddOrUpdateProject adds a new project or updates an existing one in the registry.
// It increments the project's usage count and updates the last access time.
func (r *ProjectRegistry) AddOrUpdateProject(info ProjectInfo) {
	if info.RootPath == "" {
		return // Cannot add a project without a root path
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existingInfo, found := r.Projects[info.RootPath]
	if found {
		// Update existing project
		existingInfo.UsageCount++
		existingInfo.LastAccessTime = time.Now().Unix()
		// Preserve existing environments if not provided in new info
		if info.Environments != nil {
			existingInfo.Environments = info.Environments
		}
		// Update other fields if they changed (e.g., name, type based on new detection)
		existingInfo.Name = info.Name
		existingInfo.Type = info.Type
		existingInfo.PackageInfo = info.PackageInfo
		existingInfo.Dependencies = info.Dependencies
		existingInfo.DevDependencies = info.DevDependencies
		existingInfo.DetectedPackages = info.DetectedPackages
		existingInfo.GitInfo = info.GitInfo
		// --- DO NOT update CommandHistory here ---
		// CommandHistory should only be updated by RunCommand after execution.
		// if info.CommandHistory != nil {
		// 	existingInfo.CommandHistory = info.CommandHistory
		// }
		r.Projects[info.RootPath] = existingInfo
	} else {
		// Add new project
		info.UsageCount = 1
		info.LastAccessTime = time.Now().Unix()
		if info.Environments == nil {
			info.Environments = make(map[string]EnvironmentConfig)
		}
		if info.CommandHistory == nil {
			info.CommandHistory = []HistoricCommand{}
		}
		// Ensure GeneratedFiles slice is initialized within each history entry if needed
		// (Although RunCommand should provide it)
		for i := range info.CommandHistory {
			if info.CommandHistory[i].GeneratedFiles == nil {
				info.CommandHistory[i].GeneratedFiles = []string{}
			}
		}
		r.Projects[info.RootPath] = info
	}

	// Update global stats
	r.GlobalUsages++
	r.LastUsedPath = info.RootPath

	// Consider saving immediately or batching saves
	// go r.Save() // Example: save in background (handle errors appropriately)
}

// GetProject retrieves project info by its root path.
func (r *ProjectRegistry) GetProject(rootPath string) (ProjectInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, found := r.Projects[rootPath]
	return info, found
}

// IsSubdirectoryOfProject checks if the given path is within any known project.
// Returns the parent ProjectInfo and true if it's a subdirectory, otherwise false.
func (r *ProjectRegistry) IsSubdirectoryOfProject(currentPath string) (ProjectInfo, bool) {
	absCurrentPath, err := filepath.Abs(currentPath)
	if err != nil {
		return ProjectInfo{}, false // Cannot determine absolute path
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for rootPath, info := range r.Projects {
		// Ensure comparison is done with absolute paths and separators match OS
		rel, err := filepath.Rel(rootPath, absCurrentPath)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			// currentPath is inside or is the same as rootPath
			// We check rel != "." to ensure it's a strict subdirectory
			return info, true
		}
	}
	return ProjectInfo{}, false
}

// --- Add other necessary methods for managing the registry ---
