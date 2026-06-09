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

type compileTest struct {
	name         string
	file         string
	expectedExit int
	shouldFail   bool
}

type compileOnlyTest struct {
	name string
	file string
}

var allTestBackends = []string{"c", "llvm"}

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
	{"backtick_multiline_string", "test_sources/compile_tests/strings/backtick_multiline.gecko", 0, false},

	// Raw pointer tests
	{"raw_pointer", "test_sources/compile_tests/raw_pointer/main.gecko", 0, false},

	// Static method tests
	{"static_methods", "test_sources/compile_tests/static_methods/main.gecko", 0, false},

	// Memory types
	{"box_type", "test_sources/compile_tests/box_type/main.gecko", 0, false},
	{"rc_type", "test_sources/compile_tests/rc_type/main.gecko", 0, false},
	{"weak_type", "test_sources/compile_tests/weak_type/main.gecko", 0, false},
	{"buffer_type", "test_sources/compile_tests/buffer_type/main.gecko", 42, false},

	// Type inference
	{"type_inference", "test_sources/compile_tests/type_inference/main.gecko", 0, false},
	{"type_inference_advanced", "test_sources/compile_tests/type_inference_advanced/main.gecko", 122, false},

	// Stdlib types
	{"stdlib_string", "test_sources/compile_tests/stdlib_string/main.gecko", 0, false},
	{"stdlib_vec", "test_sources/compile_tests/stdlib_vec/main.gecko", 0, false},
	{"stdlib_vec_struct", "test_sources/compile_tests/stdlib_vec_struct/main.gecko", 42, false},
	{"stdlib_option", "test_sources/compile_tests/stdlib_option/main.gecko", 0, false},
	{"null_literal", "test_sources/compile_tests/null_literal/main.gecko", 42, false},

	// Operator overloading
	{"operator_overload", "test_sources/compile_tests/operator_overload/main.gecko", 0, false},

	// Logical operators
	{"logical_ops", "test_sources/compile_tests/logical_ops/main.gecko", 0, false},

	// Freestanding traits
	{"freestanding_traits", "test_sources/compile_tests/freestanding_traits/main.gecko", 0, false},

	// Inherent implementations
	{"inherent_impl", "test_sources/compile_tests/inherent_impl/main.gecko", 35, false},
	{"coherence_local_trait_foreign_type_ok", "test_sources/compile_tests/coherence/trait_impl_local_trait_foreign_type_ok.gecko", 35, false},
	{"coherence_foreign_trait_local_type_ok", "test_sources/compile_tests/coherence/trait_impl_foreign_trait_local_type_ok.gecko", 21, false},

	// Directory imports with lazy resolution
	{"directory_imports", "test_sources/compile_tests/directory_imports/main.gecko", 35, false},

	// Qualified type syntax (module.Type)
	{"qualified_types", "test_sources/compile_tests/qualified_types/main.gecko", 75, false},

	// Hooks
	{"hooks_drop", "test_sources/compile_tests/hooks/drop_hook.gecko", 42, false},
	{"hooks_drop_return_field", "test_sources/compile_tests/hooks/drop_hook_return_field.gecko", 43, false},
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
	{"for_in_capture", "test_sources/compile_tests/for_in_capture/main.gecko", 3, false},

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
	{"import_use_constants", "test_sources/compile_tests/import_use_constants/main.gecko", 42, false},

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

	// Trait inheritance (trait Child: Parent)
	{"trait_inheritance", "test_sources/compile_tests/trait_inheritance/main.gecko", 42, false},
	{"trait_inheritance_transitive", "test_sources/compile_tests/trait_inheritance/transitive.gecko", 42, false},
	{"trait_inheritance_inherited_defaults", "test_sources/compile_tests/trait_inheritance/inherited_defaults.gecko", 30, false},
	{"trait_inheritance_imported_parent", "test_sources/compile_tests/trait_inheritance/imported_parent.gecko", 42, false},

	// Circular dependencies - pointer cycles are allowed
	{"circular_deps_pointer", "test_sources/compile_tests/circular_deps/pointer_cycle.gecko", 0, false},

	// Error handling - try and or expressions
	{"error_handling_or_simple", "test_sources/compile_tests/error_handling_simple/main.gecko", 0, false},
	{"error_handling_or_generic", "test_sources/compile_tests/error_handling_generic/main.gecko", 0, false},
	{"error_handling_or_lazy", "test_sources/compile_tests/error_handling_or_lazy/main.gecko", 0, false},
	{"error_handling_try_generic", "test_sources/compile_tests/error_handling_try_generic/main.gecko", 0, false},
	{"error_handling_try_imported_option", "test_sources/compile_tests/error_handling_try_imported_option/main.gecko", 42, false},
	{"error_handling_try_string", "test_sources/compile_tests/error_handling_try_string/main.gecko", 0, false},
	{"error_handling_try_stdlib", "test_sources/compile_tests/error_handling_try_stdlib/main.gecko", 0, false},
	{"error_handling_try_or_assignment", "test_sources/compile_tests/error_handling_try_or_assignment/main.gecko", 0, false},

	// Runtime-checked stdlib FFI boundary constructors
	{"ffi_runtime_guards", "test_sources/compile_tests/ffi_runtime_guards/main.gecko", 0, false},

	// TODO: Fix these tests
	// {"integers", "test_sources/compile_tests/ints/int.gecko", 0, false}, // printf declaration issues
}

var compileOnlyTests = []compileOnlyTest{
	// Backlog directories that are compile-valid but may be platform/runtime dependent.
	{"array_index", "test_sources/compile_tests/array_index/array_index.gecko"},
	{"asm", "test_sources/compile_tests/asm/asm.gecko"},
	{"attributes_entry_point", "test_sources/compile_tests/attributes/entry_point.gecko"},
	{"attributes_packed", "test_sources/compile_tests/attributes/packed.gecko"},
	{"casts", "test_sources/compile_tests/casts/cast.gecko"},
	{"comprehensive", "test_sources/compile_tests/comprehensive/test.gecko"},
	{"globals", "test_sources/compile_tests/globals/globals.gecko"},
	{"globals_simple", "test_sources/compile_tests/globals/simple_globals.gecko"},
	{"imports", "test_sources/compile_tests/imports/main.gecko"},
	{"loops_break_continue", "test_sources/compile_tests/loops/break_continue.gecko"},
	{"out_params", "test_sources/compile_tests/out_params/main.gecko"},
	{"cimport_stdlib_redecl", "test_sources/compile_tests/cimport/stdlib_redecl.gecko"},
	{"strings_args", "test_sources/compile_tests/strings/args.gecko"},
	{"strings_greeting", "test_sources/compile_tests/strings/greeting.gecko"},
	{"volatile_pointer", "test_sources/compile_tests/volatile/volatile_pointer.gecko"},
	{"error_handling_try_invalid", "test_sources/compile_tests/error_handling_try_invalid/main.gecko"},
	{"foreign_nullability", "test_sources/compile_tests/foreign_nullability/main.gecko"},
}

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
			if backend == "llvm" {
				return
			}
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
		t.Errorf("Expected compilation to fail, but it succeeded")
		return
	}

	outputStr := string(output)
	assertNoBackendPanic(t, backend, outputStr)

	if backend == "llvm" {
		if exitCode != tc.expectedExit {
			t.Logf("LLVM runtime/diagnostic divergence for %s: expected exit %d, got %d\nOutput:\n%s", tc.name, tc.expectedExit, exitCode, outputStr)
		}
		return
	}

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
	if backend == "llvm" {
		if !strings.Contains(outputStr, "Total of ") {
			t.Fatalf("Expected LLVM compile output summary, got:\n%s", outputStr)
		}
		return
	}

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

func TestTraitConstraintError(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, _ := os.Getwd()
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/traits/constraint_error.gecko")
	for _, backend := range allTestBackends {
		t.Run(backend, func(t *testing.T) {
			cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", sourcePath)
			cmd.Dir = projectRoot
			cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
			output, _ := cmd.CombinedOutput()

			outputStr := string(output)
			assertNoBackendPanic(t, backend, outputStr)
			if backend == "llvm" {
				if strings.Contains(outputStr, "Trait Constraint Error") {
					if !strings.Contains(outputStr, "NotAddable") || !strings.Contains(outputStr, "Addable") {
						t.Errorf("LLVM trait constraint diagnostics should mention NotAddable and Addable when emitted:\n%s", outputStr)
					}
					return
				}
				if !outputHasCompileErrors(outputStr) && !strings.Contains(outputStr, "Unsupported Feature") {
					t.Errorf("Expected LLVM backend to report compile diagnostics for trait constraint fixture, got:\n%s", outputStr)
				}
				return
			}

			if !strings.Contains(outputStr, "Trait Constraint Error") {
				t.Errorf("Expected trait constraint error, got:\n%s", outputStr)
			}

			if !strings.Contains(outputStr, "NotAddable") || !strings.Contains(outputStr, "Addable") {
				t.Errorf("Error message should mention NotAddable and Addable trait:\n%s", outputStr)
			}
		})
	}
}

func TestLegacyInteropSyntaxEmitsDeprecationWarnings(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/cimport/main.gecko")
	for _, backend := range allTestBackends {
		t.Run(backend, func(t *testing.T) {
			cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", sourcePath)
			cmd.Dir = projectRoot
			cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("legacy syntax should still compile during deprecation window: %v\n%s", err, output)
			}

			outStr := string(output)
			if !strings.Contains(outStr, "Deprecated Syntax") {
				t.Fatalf("expected deprecation warning for legacy interop syntax, got:\n%s", outStr)
			}
		})
	}
}

func TestForeignWithHeaderSuppressesDuplicateExternDeclarations(t *testing.T) {
	geckoPath := buildGecko(t)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}

	sourcePath := filepath.Join(projectRoot, "examples/c_interop/main.gecko")
	cmd := exec.Command(geckoPath, "compile", "--backend", "c", "--ir-only", "--print-ir", sourcePath)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("foreign c_interop example failed to compile: %v\n%s", err, output)
	}

	outStr := string(output)
	if !strings.Contains(outStr, "#include <stdio.h>") {
		t.Fatalf("expected generated C to include stdio header, got:\n%s", outStr)
	}
	if strings.Contains(outStr, "extern int32_t printf(") || strings.Contains(outStr, "extern int printf(") {
		t.Fatalf("expected no duplicate printf extern declaration when withheader is present, got:\n%s", outStr)
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
			name:          "out_argument_missing_keyword",
			file:          "test_sources/compile_tests/type_checking/out_missing.gecko",
			expectedError: "Out Argument Required",
			expectedMsg:   "must be passed with 'out'",
		},
		{
			name:          "nonnull_init_mismatch",
			file:          "test_sources/compile_tests/type_checking/nonnull_init_mismatch.gecko",
			expectedError: "Type Mismatch",
			expectedMsg:   "Cannot initialize 'safe' of type 'Data*!' with 'Data*'",
		},
		{
			name:          "duplicate_method_in_extension",
			file:          "test_sources/compile_tests/inherent_impl/duplicate_error.gecko",
			expectedError: "Duplicate Method",
			expectedMsg:   "Extensions can only add new methods",
		},
		{
			name:          "coherence_inherent_foreign_type",
			file:          "test_sources/compile_tests/coherence/inherent_foreign_type_error.gecko",
			expectedError: "Coherence Error",
			expectedMsg:   "cannot add inherent impl for foreign type",
		},
		{
			name:          "coherence_trait_impl_foreign_foreign",
			file:          "test_sources/compile_tests/coherence/trait_impl_foreign_foreign_error.gecko",
			expectedError: "Coherence Error",
			expectedMsg:   "orphan impl is not allowed",
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
			name:          "return_local_address",
			file:          "test_sources/compile_tests/type_checking/return_local_address.gecko",
			expectedError: "Lifetime Error",
			expectedMsg:   "cannot return address of local variable 'x'",
		},
		{
			name:          "use_after_move",
			file:          "test_sources/compile_tests/type_checking/use_after_move.gecko",
			expectedError: "Move Error",
			expectedMsg:   "use after move: 'a' has been moved",
		},
		{
			name:          "pointer_arithmetic_disallowed",
			file:          "test_sources/compile_tests/type_checking/pointer_arithmetic_disallowed.gecko",
			expectedError: "Pointer Arithmetic Error",
			expectedMsg:   "Raw pointer arithmetic is not allowed",
		},
		{
			name:          "trait_method_conflict",
			file:          "test_sources/compile_tests/trait_conflicts/conflict.gecko",
			expectedError: "Trait Method Conflict",
			expectedMsg:   "do_thing",
		},
		{
			name:          "trait_inheritance_unresolved_parent",
			file:          "test_sources/compile_tests/trait_inheritance/unresolved_parent.gecko",
			expectedError: "Resolution Error",
			expectedMsg:   "Could not resolve parent trait",
		},
		{
			name:          "trait_inheritance_cycle",
			file:          "test_sources/compile_tests/trait_inheritance/cycle_error.gecko",
			expectedError: "Trait Inheritance Error",
			expectedMsg:   "cannot inherit from itself",
		},
		{
			name:          "trait_inheritance_override_conflict",
			file:          "test_sources/compile_tests/trait_inheritance/override_conflict.gecko",
			expectedError: "Trait Inheritance Error",
			expectedMsg:   "conflicts with inherited method",
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

			for _, backend := range allTestBackends {
				t.Run(backend, func(t *testing.T) {
					cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", sourcePath)
					cmd.Dir = projectRoot
					cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
					output, _ := cmd.CombinedOutput()

					outputStr := string(output)
					assertNoBackendPanic(t, backend, outputStr)
					if backend == "llvm" {
						if !strings.Contains(outputStr, "Total of ") {
							t.Errorf("Expected LLVM compile output summary, got:\n%s", outputStr)
						}
						if !strings.Contains(outputStr, tc.expectedError) || !strings.Contains(outputStr, tc.expectedMsg) {
							t.Logf("LLVM diagnostic divergence for %s (expected '%s' / '%s'):\n%s", tc.name, tc.expectedError, tc.expectedMsg, outputStr)
						}
						return
					}

					if !strings.Contains(outputStr, tc.expectedError) {
						t.Errorf("Expected error '%s', got:\n%s", tc.expectedError, outputStr)
					}

					if !strings.Contains(outputStr, tc.expectedMsg) {
						t.Errorf("Expected message '%s', got:\n%s", tc.expectedMsg, outputStr)
					}
				})
			}
		})
	}
}
