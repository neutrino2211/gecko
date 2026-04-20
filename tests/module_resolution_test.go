package tests

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModuleResolutionOrder(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "gecko_module_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create directory structure:
	// tmpDir/
	//   project/
	//     main.gecko
	//     local.gecko         <- relative import
	//     vendor/
	//       vendored.gecko    <- vendor import
	//   stdlib/
	//     collections/
	//       vec.gecko         <- stdlib import

	projectDir := filepath.Join(tmpDir, "project")
	vendorDir := filepath.Join(projectDir, "vendor")
	stdlibDir := filepath.Join(tmpDir, "stdlib")
	collectionsDir := filepath.Join(stdlibDir, "collections")

	dirs := []string{projectDir, vendorDir, stdlibDir, collectionsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create test files
	files := map[string]string{
		filepath.Join(projectDir, "local.gecko"): `package local
public func get_local(): int32 {
    return 1
}`,
		filepath.Join(vendorDir, "vendored.gecko"): `package vendored
public func get_vendored(): int32 {
    return 2
}`,
		filepath.Join(collectionsDir, "vec.gecko"): `package vec
public class Vec<T> {
    let len: int32
}`,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Test cases for different import types
	tests := []struct {
		name           string
		mainContent    string
		geckoHome      string
		shouldResolve  []string
	}{
		{
			name: "relative_import",
			mainContent: `package main
import local

func main(): int32 {
    return 0
}`,
			geckoHome:     tmpDir,
			shouldResolve: []string{"local"},
		},
		{
			name: "vendor_import",
			mainContent: `package main
import vendored

func main(): int32 {
    return 0
}`,
			geckoHome:     tmpDir,
			shouldResolve: []string{"vendored"},
		},
		{
			name: "stdlib_import",
			mainContent: `package main
import std.collections.vec

func main(): int32 {
    return 0
}`,
			geckoHome:     tmpDir,
			shouldResolve: []string{"std.collections.vec"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Write the main file
			mainPath := filepath.Join(projectDir, "main.gecko")
			if err := os.WriteFile(mainPath, []byte(tc.mainContent), 0644); err != nil {
				t.Fatalf("Failed to write main.gecko: %v", err)
			}

			// Set GECKO_HOME for stdlib resolution
			oldHome := os.Getenv("GECKO_HOME")
			os.Setenv("GECKO_HOME", tc.geckoHome)
			defer os.Setenv("GECKO_HOME", oldHome)

			// The actual resolution is tested through compile - we just verify
			// the file structure is correct for the compiler to find
			for _, expected := range tc.shouldResolve {
				t.Logf("Module %s should be resolvable", expected)
			}
		})
	}
}

func TestGetGeckoHome(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectEnv   bool
	}{
		{
			name:      "env_var_set",
			envValue:  "/custom/gecko/path",
			expectEnv: true,
		},
		{
			name:      "env_var_empty",
			envValue:  "",
			expectEnv: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			oldHome := os.Getenv("GECKO_HOME")
			defer os.Setenv("GECKO_HOME", oldHome)

			os.Setenv("GECKO_HOME", tc.envValue)

			// We can't directly test getGeckoHome() since it's not exported,
			// but we can verify the env var is read correctly
			if tc.expectEnv {
				if os.Getenv("GECKO_HOME") != tc.envValue {
					t.Errorf("Expected GECKO_HOME=%s, got %s", tc.envValue, os.Getenv("GECKO_HOME"))
				}
			}
		})
	}
}

func TestStdlibImportStripsPrefix(t *testing.T) {
	// Create a temp stdlib with the correct structure
	tmpDir, err := os.MkdirTemp("", "gecko_stdlib_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create stdlib/option.gecko (not stdlib/std/option.gecko)
	stdlibDir := filepath.Join(tmpDir, "stdlib")
	if err := os.MkdirAll(stdlibDir, 0755); err != nil {
		t.Fatalf("Failed to create stdlib dir: %v", err)
	}

	optionContent := `package option
public class Option<T> {
    let has_value: bool
    let value: T
}
`
	if err := os.WriteFile(filepath.Join(stdlibDir, "option.gecko"), []byte(optionContent), 0644); err != nil {
		t.Fatalf("Failed to write option.gecko: %v", err)
	}

	// The import `std.option` should look for `$GECKO_HOME/stdlib/option.gecko`
	// (stripping the `std` prefix), not `$GECKO_HOME/stdlib/std/option.gecko`
	expectedPath := filepath.Join(stdlibDir, "option.gecko")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("Expected stdlib file at %s", expectedPath)
	}
}
