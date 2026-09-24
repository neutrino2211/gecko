// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var errorsGeneratedPattern = regexp.MustCompile(`\b([0-9]+) errors generated\b`)

func outputHasCompileErrors(outputStr string) bool {
	matches := errorsGeneratedPattern.FindAllStringSubmatch(outputStr, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		errorCount, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if errorCount > 0 {
			return true
		}
	}
	return false
}

func assertNoBackendPanic(t *testing.T, backend string, outputStr string) {
	t.Helper()
	if strings.Contains(outputStr, "panic:") {
		t.Fatalf("%s backend must not panic:\n%s", backend, outputStr)
	}
}

func TestCompileAndRun(t *testing.T) {
	// Build the compiler first
	geckoPath := buildGecko(t)

	for _, tc := range compileTests {
		t.Run(tc.name, func(t *testing.T) {
			for _, backend := range allTestBackends {
				t.Run(backend, func(t *testing.T) {
					runCompileTest(t, geckoPath, tc, backend)
				})
			}
		})
	}
}

func TestCompileOnly(t *testing.T) {
	// Build the compiler first
	geckoPath := buildGecko(t)

	for _, tc := range compileOnlyTests {
		t.Run(tc.name, func(t *testing.T) {
			for _, backend := range allTestBackends {
				t.Run(backend, func(t *testing.T) {
					runCompileOnlyTest(t, geckoPath, tc, backend)
				})
			}
		})
	}
}

func TestTryDiagnosticsUsesGeckoExpression(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/error_handling_try_diagnostics/main.gecko")
	for _, backend := range allTestBackends {
		t.Run(backend, func(t *testing.T) {
			cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", "--print-ir", "--no-treeshake", sourcePath)
			cmd.Dir = projectRoot
			cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Compilation failed unexpectedly: %v\n%s", err, output)
			}

			outputStr := string(output)
			assertNoBackendPanic(t, backend, outputStr)
			if !strings.Contains(outputStr, `File::open(\"no_exist\", \"r\")`) {
				t.Fatalf("Expected try diagnostics expression to use Gecko syntax, got:\n%s", outputStr)
			}
			if strings.Contains(outputStr, `File__open(\"no_exist\", \"r\")`) {
				t.Fatalf("Expected try diagnostics expression to avoid C mangled syntax, got:\n%s", outputStr)
			}
		})
	}
}

func buildGecko(t *testing.T) string {
	t.Helper()

	// Get project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Go up from tests/ to project root
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	geckoPath := filepath.Join(os.TempDir(), "gecko_test_binary")

	cmd := exec.Command("go", "build", "-o", geckoPath, ".")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build gecko: %v\n%s", err, output)
	}

	return geckoPath
}

func runCompileTest(t *testing.T, geckoPath string, tc compileTest, backend string) {
	t.Helper()

	// Get project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, tc.file)

	// Run gecko run command
	cmd := exec.Command(geckoPath, "run", "--backend", backend, sourcePath)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			if tc.shouldFail {
				return // Expected failure
			}
			t.Fatalf("Command failed unexpectedly: %v\n%s", err, output)
		}
	}

	if tc.shouldFail {
		assertNoBackendPanic(t, backend, string(output))
		if strings.Contains(string(output), "Unsafe Required") || strings.Contains(string(output), "Borrow Error") || strings.Contains(string(output), "Move Error") || outputHasCompileErrors(string(output)) {
			return
		}
		t.Errorf("Expected compilation to fail, but it succeeded")
		return
	}

	outputStr := string(output)
	assertNoBackendPanic(t, backend, outputStr)

	if exitCode != tc.expectedExit {
		t.Errorf("Expected exit code %d, got %d\nOutput:\n%s", tc.expectedExit, exitCode, output)
	}

	// Check for unexpected errors in output (excluding known type resolution warnings)
	if strings.Contains(outputStr, "error:") || strings.Contains(outputStr, "Error:") {
		// Filter out known benign errors
		if !strings.Contains(outputStr, "Type Check Error: Unable to resolve type") {
			t.Logf("Warning: Output contains errors:\n%s", outputStr)
		}
	}
}

func runCompileOnlyTest(t *testing.T, geckoPath string, tc compileOnlyTest, backend string) {
	t.Helper()

	// Get project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, tc.file)

	cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", sourcePath)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Compilation failed unexpectedly: %v\n%s", err, output)
	}

	outputStr := string(output)
	assertNoBackendPanic(t, backend, outputStr)

	matches := errorsGeneratedPattern.FindAllStringSubmatch(outputStr, -1)
	if len(matches) == 0 {
		// Fallback guard in case summary format changes.
		if strings.Contains(outputStr, "error:") || strings.Contains(outputStr, "Error:") {
			t.Fatalf("Compilation output contains errors:\n%s", outputStr)
		}
		return
	}

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		errorCount, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			t.Fatalf("Failed to parse error count from output:\n%s", outputStr)
		}
		if errorCount > 0 {
			t.Fatalf("Expected zero compile errors, got %d\nOutput:\n%s", errorCount, outputStr)
		}
	}
}
