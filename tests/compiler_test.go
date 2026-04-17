package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type compileTest struct {
	name         string
	file         string
	expectedExit int
	shouldFail   bool
}

var compileTests = []compileTest{
	// Trait tests
	{"traits_basic", "test_sources/compile_tests/traits/basic.gecko", 30, false},
	{"traits_constraints", "test_sources/compile_tests/traits/constraints.gecko", 42, false},

	// Generic tests
	{"generics_containers", "test_sources/compile_tests/generics/containers.gecko", 19, false},

	// Pointer tests
	{"pointers_nonnull", "test_sources/compile_tests/pointers/nonnull.gecko", 42, false},

	// External type tests
	{"external_types_basic", "test_sources/compile_tests/external_types/basic.gecko", 55, false},

	// Intrinsics tests
	{"intrinsics_basic", "test_sources/compile_tests/intrinsics/basic.gecko", 17, false},

	// Builtin traits tests
	{"builtin_traits_pointer", "test_sources/compile_tests/builtin_traits/pointer.gecko", 43, false},
	{"builtin_traits_nonnull", "test_sources/compile_tests/builtin_traits/nonnull.gecko", 40, false},

	// String module tests
	{"string_builder", "test_sources/compile_tests/string_builder/main.gecko", 0, false},

	// Raw pointer tests
	{"raw_pointer", "test_sources/compile_tests/raw_pointer/main.gecko", 0, false},

	// Static method tests
	{"static_methods", "test_sources/compile_tests/static_methods/main.gecko", 0, false},

	// Memory types
	{"box_type", "test_sources/compile_tests/box_type/main.gecko", 0, false},
	{"rc_type", "test_sources/compile_tests/rc_type/main.gecko", 0, false},
	{"weak_type", "test_sources/compile_tests/weak_type/main.gecko", 0, false},

	// Type inference
	{"type_inference", "test_sources/compile_tests/type_inference/main.gecko", 0, false},

	// Stdlib types
	{"stdlib_string", "test_sources/compile_tests/stdlib_string/main.gecko", 0, false},
	{"stdlib_vec", "test_sources/compile_tests/stdlib_vec/main.gecko", 0, false},
	{"stdlib_option", "test_sources/compile_tests/stdlib_option/main.gecko", 0, false},

	// Operator overloading
	{"operator_overload", "test_sources/compile_tests/operator_overload/main.gecko", 0, false},

	// Logical operators
	{"logical_ops", "test_sources/compile_tests/logical_ops/main.gecko", 0, false},

	// Freestanding traits
	{"freestanding_traits", "test_sources/compile_tests/freestanding_traits/main.gecko", 0, false},

	// Examples
	{"example_traits", "examples/traits/shapes.gecko", 93, false},
	{"example_stdlib", "examples/stdlib/demo.gecko", 80, false},

	// TODO: Fix these tests
	// {"integers", "test_sources/compile_tests/ints/int.gecko", 0, false}, // printf declaration issues
}

func TestCompileAndRun(t *testing.T) {
	// Build the compiler first
	geckoPath := buildGecko(t)

	for _, tc := range compileTests {
		t.Run(tc.name, func(t *testing.T) {
			runCompileTest(t, geckoPath, tc)
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

func runCompileTest(t *testing.T, geckoPath string, tc compileTest) {
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
	cmd := exec.Command(geckoPath, "run", sourcePath)
	cmd.Dir = projectRoot
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
		t.Errorf("Expected compilation to fail, but it succeeded")
		return
	}

	if exitCode != tc.expectedExit {
		t.Errorf("Expected exit code %d, got %d\nOutput:\n%s", tc.expectedExit, exitCode, output)
	}

	// Check for unexpected errors in output (excluding known type resolution warnings)
	outputStr := string(output)
	if strings.Contains(outputStr, "error:") || strings.Contains(outputStr, "Error:") {
		// Filter out known benign errors
		if !strings.Contains(outputStr, "Type Check Error: Unable to resolve type") {
			t.Logf("Warning: Output contains errors:\n%s", outputStr)
		}
	}
}

func TestTraitConstraintError(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/traits/constraint_error.gecko")

	cmd := exec.Command(geckoPath, "compile", "--backend", "c", "--ir-only", sourcePath)
	cmd.Dir = projectRoot
	output, _ := cmd.CombinedOutput()

	outputStr := string(output)
	if !strings.Contains(outputStr, "Trait Constraint Error") {
		t.Errorf("Expected trait constraint error, got:\n%s", outputStr)
	}

	if !strings.Contains(outputStr, "NotAddable") || !strings.Contains(outputStr, "Addable") {
		t.Errorf("Error message should mention NotAddable and Addable trait:\n%s", outputStr)
	}
}
