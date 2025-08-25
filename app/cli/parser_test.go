package cli

import (
	"reflect"
	"testing"
)

// MockRegistryChecker provides a mock implementation for testing.
type MockRegistryChecker struct {
	KnownCommands map[string]bool
}

// CommandExists checks if a command name exists in the mock registry.
func (m MockRegistryChecker) CommandExists(name string) bool {
	_, exists := m.KnownCommands[name]
	return exists
}

// TestParseCommandLineArgs tests the argument parser.
func TestParseCommandLineArgs(t *testing.T) {
	// Setup a mock registry for testing command existence
	mockRegistry := MockRegistryChecker{
		KnownCommands: map[string]bool{
			"hello":           true,
			"commands":        true,
			"list-all":        true,
			"stats":           true,
			"config set":      true, // Multi-word
			"config get":      true,
			"config list":     true,
			"clipboard-paste": true,
			"native-cmd add":  true,
			// Add other multi-word commands as needed
		},
	}

	testCases := []struct {
		name     string
		args     []string
		expected CommandArgs
	}{
		{
			name: "No Args",
			args: []string{},
			expected: CommandArgs{
				RawArgs:   []string{},
				Variables: []string{},
				Flags:     map[string]string{},
				BoolFlags: map[string]bool{},
				Errors:    []error{},
			},
		},
		{
			name: "Version Flag",
			args: []string{"--version"},
			expected: CommandArgs{
				RawArgs:          []string{"--version"},
				VersionRequested: true,
				Variables:        []string{},
				Flags:            map[string]string{},
				BoolFlags:        map[string]bool{},
				Errors:           []error{},
			},
		},
		{
			name: "General Help Flag",
			args: []string{"--help"},
			expected: CommandArgs{
				RawArgs:       []string{"--help"},
				HelpRequested: true,
				Variables:     []string{},
				Flags:         map[string]string{},
				BoolFlags:     map[string]bool{"help": true},
				Errors:        []error{},
			},
		},
		{
			name: "Command Specific Help",
			args: []string{"hello", "--help"},
			expected: CommandArgs{
				RawArgs:       []string{"hello", "--help"},
				CommandName:   "hello",
				HelpRequested: true,
				Variables:     []string{},
				Flags:         map[string]string{},
				BoolFlags:     map[string]bool{"help": true},
				Errors:        []error{},
			},
		},
		{
			name: "Multi-word Command Specific Help",
			args: []string{"config", "set", "--help"},
			expected: CommandArgs{
				RawArgs:       []string{"config", "set", "--help"},
				CommandName:   "config set",
				HelpRequested: true,
				Variables:     []string{},
				Flags:         map[string]string{},
				BoolFlags:     map[string]bool{"help": true},
				Errors:        []error{},
			},
		},
		{
			name: "Simple Command",
			args: []string{"commands"},
			expected: CommandArgs{
				RawArgs:     []string{"commands"},
				CommandName: "commands",
				Variables:   []string{},
				Flags:       map[string]string{},
				BoolFlags:   map[string]bool{},
				Errors:      []error{},
			},
		},
		{
			name: "Command with Variables and Flags",
			args: []string{"clipboard-paste", "item1", "./out.txt", "--overwrite", "-f", "--target-dir=./data"},
			expected: CommandArgs{
				RawArgs:     []string{"clipboard-paste", "item1", "./out.txt", "--overwrite", "-f", "--target-dir=./data"},
				CommandName: "clipboard-paste",
				Variables:   []string{"item1", "./out.txt"},
				Flags:       map[string]string{"target-dir": "./data"},
				BoolFlags:   map[string]bool{"overwrite": true, "f": true},
				// Note: Duplicate flags (-f/--overwrite might be handled by execution logic, not parser)
				Errors: []error{},
			},
		},
		{
			name: "Multi-word Command with Variables and Flag",
			args: []string{"config", "set", "myKey", "myValue", "--global"},
			expected: CommandArgs{
				RawArgs:     []string{"config", "set", "myKey", "myValue", "--global"},
				CommandName: "config set",
				Variables:   []string{"myKey", "myValue"},
				Flags:       map[string]string{},
				BoolFlags:   map[string]bool{"global": true},
				Errors:      []error{},
			},
		},
		{
			name: "Flag with Space Value",
			args: []string{"config", "get", "myKey", "--output", "file.txt"},
			expected: CommandArgs{
				RawArgs:     []string{"config", "get", "myKey", "--output", "file.txt"},
				CommandName: "config get",
				Variables:   []string{"myKey"},
				Flags:       map[string]string{"output": "file.txt"},
				BoolFlags:   map[string]bool{},
				Errors:      []error{},
			},
		},
		{
			name: "Combined Short Flags",
			args: []string{"cmd", "-abc", "valueForC"},
			// Assuming "cmd" is NOT a registered command
			expected: CommandArgs{
				RawArgs:     []string{"cmd", "-abc", "valueForC"},
				CommandName: "", // cmd is treated as variable
				Variables:   []string{"cmd"},
				Flags:       map[string]string{"c": "valueForC"},
				BoolFlags:   map[string]bool{"a": true, "b": true},
				Errors:      []error{},
			},
		},
		{
			name: "Unknown Command",
			args: []string{"unknowncmd", "arg1"},
			expected: CommandArgs{
				RawArgs:     []string{"unknowncmd", "arg1"},
				CommandName: "",                             // Not found in registry
				Variables:   []string{"unknowncmd", "arg1"}, // Treated as variables
				Flags:       map[string]string{},
				BoolFlags:   map[string]bool{},
				Errors:      []error{},
			},
		},
		// Add more cases: invalid flags, duplicate flags, edge cases
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Pass the mock registry to the parser
			actual := ParseCommandLineArgs(tc.args, mockRegistry)

			// Compare fields individually for better error messages
			if actual.CommandName != tc.expected.CommandName {
				t.Errorf("CommandName mismatch: expected %q, got %q", tc.expected.CommandName, actual.CommandName)
			}
			if !reflect.DeepEqual(actual.Variables, tc.expected.Variables) {
				t.Errorf("Variables mismatch: expected %v, got %v", tc.expected.Variables, actual.Variables)
			}
			if !reflect.DeepEqual(actual.Flags, tc.expected.Flags) {
				t.Errorf("Flags mismatch: expected %v, got %v", tc.expected.Flags, actual.Flags)
			}
			if !reflect.DeepEqual(actual.BoolFlags, tc.expected.BoolFlags) {
				t.Errorf("BoolFlags mismatch: expected %v, got %v", tc.expected.BoolFlags, actual.BoolFlags)
			}
			if actual.HelpRequested != tc.expected.HelpRequested {
				t.Errorf("HelpRequested mismatch: expected %t, got %t", tc.expected.HelpRequested, actual.HelpRequested)
			}
			if actual.VersionRequested != tc.expected.VersionRequested {
				t.Errorf("VersionRequested mismatch: expected %t, got %t", tc.expected.VersionRequested, actual.VersionRequested)
			}

			// Basic error count check (improve by checking specific errors if needed)
			if len(actual.Errors) != len(tc.expected.Errors) {
				t.Errorf("Errors length mismatch: expected %d, got %d (Errors: %v)", len(tc.expected.Errors), len(actual.Errors), actual.Errors)
			}
		})
	}
}
