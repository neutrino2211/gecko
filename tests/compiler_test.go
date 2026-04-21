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
	{"type_inference_advanced", "test_sources/compile_tests/type_inference_advanced/main.gecko", 122, false},

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

	// Inherent implementations
	{"inherent_impl", "test_sources/compile_tests/inherent_impl/main.gecko", 35, false},

	// Directory imports with lazy resolution
	{"directory_imports", "test_sources/compile_tests/directory_imports/main.gecko", 35, false},

	// Qualified type syntax (module.Type)
	{"qualified_types", "test_sources/compile_tests/qualified_types/main.gecko", 75, false},

	// Hooks
	{"hooks_drop", "test_sources/compile_tests/hooks/drop_hook.gecko", 42, false},
	{"hooks_operator_add", "test_sources/compile_tests/hooks/operator_add.gecko", 42, false},
	{"hooks_operators_arithmetic", "test_sources/compile_tests/hooks/operators_arithmetic.gecko", 44, false},
	{"hooks_operators_comparison", "test_sources/compile_tests/hooks/operators_comparison.gecko", 63, false},
	{"hooks_operators_bitwise", "test_sources/compile_tests/hooks/operators_bitwise.gecko", 79, false},
	{"hooks_operators_unary", "test_sources/compile_tests/hooks/operators_unary.gecko", 42, false},

	// Examples
	{"example_traits", "examples/traits/shapes.gecko", 93, false},
	{"example_stdlib", "examples/stdlib/demo.gecko", 80, false},
	{"example_c_interop", "examples/c_interop/main.gecko", 105, false},
	{"example_string_builder", "examples/string_builder/demo.gecko", 0, false},

	// Visibility tests
	{"visibility_public_access", "test_sources/compile_tests/visibility/public_access.gecko", 42, false},

	// Index hook tests
	{"index_hook", "test_sources/compile_tests/index_hook/main.gecko", 42, false},

	// Iterator / for-in loop tests
	{"for_in_loop", "test_sources/compile_tests/for_in_loop/main.gecko", 0, false},

	// Lazy resolution tests
	{"lazy_method_resolution", "test_sources/compile_tests/lazy_method_resolution/main.gecko", 0, false},

	// Narrowing tests
	{"narrowing_test", "test_sources/compile_tests/narrowing_test/main.gecko", 0, false},

	// Type checking runtime tests
	{"type_checking_default_impl_generic", "test_sources/compile_tests/type_checking/default_impl_generic.gecko", 0, false},
	{"type_checking_default_impl_valid", "test_sources/compile_tests/type_checking/default_impl_valid.gecko", 42, false},
	{"type_checking_generic_valid", "test_sources/compile_tests/type_checking/generic_valid.gecko", 0, false},

	// C import tests
	{"cimport", "test_sources/compile_tests/cimport/main.gecko", 0, false},

	// Packed structs
	{"packed", "test_sources/compile_tests/packed/packed.gecko", 0, false},

	// Struct literals
	{"struct_literal", "test_sources/compile_tests/struct_literal/struct_literal.gecko", 0, false},
	{"struct_inline", "test_sources/compile_tests/struct_literal/struct_inline.gecko", 0, false},

	// Fixed-size arrays
	{"fixed_arrays", "test_sources/compile_tests/fixed_arrays/fixed_arrays.gecko", 0, false},

	// Type checking valid code
	{"typecheck_valid", "test_sources/compile_tests/typecheck/typecheck_valid.gecko", 0, false},

	// Enums
	{"enums", "test_sources/compile_tests/enums/main.gecko", 2, false},

	// Nested generics (3+ levels)
	{"nested_generics", "test_sources/compile_tests/nested_generics/main.gecko", 42, false},

	// Generic trait implementations (impl<T> Trait for Class<T>)
	{"generic_trait_impl", "test_sources/compile_tests/generic_trait_impl/main.gecko", 0, false},

	// String iteration
	{"string_iter", "test_sources/compile_tests/string_iter/main.gecko", 0, false},

	// Multiple trait constraints (T is A & B)
	{"multiple_constraints", "test_sources/compile_tests/multiple_constraints/main.gecko", 31, false},

	// Circular dependencies - pointer cycles are allowed
	{"circular_deps_pointer", "test_sources/compile_tests/circular_deps/pointer_cycle.gecko", 0, false},

	// Error handling - try and or expressions
	{"error_handling_or_simple", "test_sources/compile_tests/error_handling_simple/main.gecko", 0, false},
	{"error_handling_or_generic", "test_sources/compile_tests/error_handling_generic/main.gecko", 0, false},
	{"error_handling_try_generic", "test_sources/compile_tests/error_handling_try_generic/main.gecko", 0, false},

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

func TestTypeCheckingErrors(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	tests := []struct {
		name          string
		file          string
		expectedError string
		expectedMsg   string
	}{
		{
			name:          "field_type_mismatch",
			file:          "test_sources/compile_tests/type_checking/field_mismatch.gecko",
			expectedError: "Type Mismatch",
			expectedMsg:   "Cannot assign 'string' to 'c.radius' of type 'int'",
		},
		{
			name:          "variable_type_mismatch",
			file:          "test_sources/compile_tests/type_checking/var_mismatch.gecko",
			expectedError: "Type Mismatch",
			expectedMsg:   "Cannot assign 'string' to 'x' of type 'int'",
		},
		{
			name:          "const_reassignment",
			file:          "test_sources/compile_tests/type_checking/const_reassign.gecko",
			expectedError: "Constant Reassignment",
			expectedMsg:   "Cannot reassign constant 'x'",
		},
		{
			name:          "duplicate_method_in_extension",
			file:          "test_sources/compile_tests/inherent_impl/duplicate_error.gecko",
			expectedError: "Duplicate Method",
			expectedMsg:   "Extensions can only add new methods",
		},
		{
			name:          "type_suggestion",
			file:          "test_sources/compile_tests/type_suggestions/missing_type.gecko",
			expectedError: "Type Check Error",
			expectedMsg:   "std.collections.string",
		},
		{
			name:          "hook_invalid_method",
			file:          "test_sources/compile_tests/hooks/invalid_hook.gecko",
			expectedError: "Hook Signature Error",
			expectedMsg:   "not found in trait",
		},
		{
			name:          "hook_duplicate",
			file:          "test_sources/compile_tests/hooks/duplicate_hook.gecko",
			expectedError: "Duplicate Hook",
			expectedMsg:   "already registered",
		},
		{
			name:          "hook_wrong_signature",
			file:          "test_sources/compile_tests/hooks/wrong_signature.gecko",
			expectedError: "Hook Signature Error",
			expectedMsg:   "wrong return type",
		},
		{
			name:          "visibility_private_access",
			file:          "test_sources/compile_tests/visibility/private_access.gecko",
			expectedError: "Visibility Error",
			expectedMsg:   "private (default)",
		},
		{
			name:          "visibility_private_method",
			file:          "test_sources/compile_tests/visibility/private_method_access.gecko",
			expectedError: "Visibility Error",
			expectedMsg:   "method 'private_method' is private (default)",
		},
		{
			name:          "generic_type_mismatch",
			file:          "test_sources/compile_tests/type_checking/generic_mismatch.gecko",
			expectedError: "Type Mismatch",
			expectedMsg:   "expects type 'int32', got 'string'",
		},
		{
			name:          "return_type_mismatch",
			file:          "test_sources/compile_tests/type_checking/return_mismatch.gecko",
			expectedError: "Return Type Mismatch",
			expectedMsg:   "Cannot return 'string' from function expecting 'int32'",
		},
		{
			name:          "trait_method_conflict",
			file:          "test_sources/compile_tests/trait_conflicts/conflict.gecko",
			expectedError: "Trait Method Conflict",
			expectedMsg:   "do_thing",
		},
		{
			name:          "circular_type_dependency",
			file:          "test_sources/compile_tests/circular_deps/value_cycle_error.gecko",
			expectedError: "Circular Type Dependency",
			expectedMsg:   "infinite size",
		},
		{
			name:          "circular_type_dependency_three_way",
			file:          "test_sources/compile_tests/circular_deps/three_way_cycle.gecko",
			expectedError: "Circular Type Dependency",
			expectedMsg:   "infinite size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sourcePath := filepath.Join(projectRoot, tc.file)

			cmd := exec.Command(geckoPath, "compile", "--backend", "c", "--ir-only", sourcePath)
			cmd.Dir = projectRoot
			output, _ := cmd.CombinedOutput()

			outputStr := string(output)
			if !strings.Contains(outputStr, tc.expectedError) {
				t.Errorf("Expected error '%s', got:\n%s", tc.expectedError, outputStr)
			}

			if !strings.Contains(outputStr, tc.expectedMsg) {
				t.Errorf("Expected message '%s', got:\n%s", tc.expectedMsg, outputStr)
			}
		})
	}
}
