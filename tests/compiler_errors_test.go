// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileIRExitStatus(t *testing.T) {
	geckoPath := buildGecko(t)
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		source     string
		backend    string
		wantError  bool
		diagnostic string
	}{
		{"valid", "raw_access", "c", false, "0 errors generated"},
		{"invalid", "raw_deref_read_error", "c", true, "Unsafe Required"},
		{"unavailable_backend", "raw_access", "llvm", true, "Backend 'llvm' not found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sourcePath := filepath.Join(projectRoot, "test_sources", "compile_tests", tc.source, "main.gecko")
			artifactPath := filepath.Join(t.TempDir(), "main.c")
			cmd := exec.Command(geckoPath, "compile", "--backend", tc.backend, "--ir-only", "--output", artifactPath, sourcePath)
			cmd.Dir = projectRoot
			cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
			output, runErr := cmd.CombinedOutput()
			if !strings.Contains(string(output), tc.diagnostic) {
				t.Fatalf("expected %q in compiler output:\n%s", tc.diagnostic, output)
			}

			var exitErr *exec.ExitError
			if tc.wantError {
				if !errors.As(runErr, &exitErr) || exitErr.ExitCode() == 0 {
					t.Fatalf("expected nonzero exit, got %v:\n%s", runErr, output)
				}
				if !outputHasCompileErrors(string(output)) {
					t.Fatalf("expected error summary:\n%s", output)
				}
				if _, statErr := os.Stat(artifactPath); !os.IsNotExist(statErr) {
					t.Fatalf("failed compile produced artifact: %v", statErr)
				}
				return
			}

			if runErr != nil {
				t.Fatalf("expected successful compile, got %v:\n%s", runErr, output)
			}
			if _, statErr := os.Stat(artifactPath); statErr != nil {
				t.Fatalf("expected IR artifact: %v", statErr)
			}
		})
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
			name:          "unsafe_setup_scope",
			file:          "test_sources/compile_tests/unsafe_setup_scope_error/main.gecko",
			expectedError: "Scope Error",
			expectedMsg:   "hidden",
		},
		{
			name:          "unsafe_setup_forward",
			file:          "test_sources/compile_tests/unsafe_setup_forward_error/main.gecko",
			expectedError: "Scope Error",
			expectedMsg:   "later",
		},
		{
			name:          "unsafe_setup_handler",
			file:          "test_sources/compile_tests/unsafe_setup_handler_error/main.gecko",
			expectedError: "Unsafe Handler Error",
			expectedMsg:   "must implement UnsafeHandler",
		},
		{
			name:          "unsafe_setup_nested_alloc",
			file:          "test_sources/compile_tests/unsafe_setup_nested_alloc_error/main.gecko",
			expectedError: "Unsafe Setup Error",
			expectedMsg:   "entire setup initializer",
		},
		{
			name:          "unsafe_alloc_type",
			file:          "test_sources/compile_tests/unsafe_alloc_type_error/main.gecko",
			expectedError: "Intrinsic Error",
			expectedMsg:   "must be integers",
		},
		{
			name:          "unsafe_setup_duplicate",
			file:          "test_sources/compile_tests/unsafe_setup_duplicate_error/main.gecko",
			expectedError: "Unsafe Setup Error",
			expectedMsg:   "Duplicate setup binding",
		},
		{
			name:          "unsafe_setup_access",
			file:          "test_sources/compile_tests/unsafe_setup_access_error/main.gecko",
			expectedError: "Unsafe Setup Error",
			expectedMsg:   "after setup validation",
		},
		{
			name:          "unsafe_alloc_context",
			file:          "test_sources/compile_tests/unsafe_alloc_context_error/main.gecko",
			expectedError: "Unsafe Required",
			expectedMsg:   "@alloc is only allowed",
		},
		{
			name:          "unsafe_free_context",
			file:          "test_sources/compile_tests/unsafe_free_context_error/main.gecko",
			expectedError: "Unsafe Required",
			expectedMsg:   "@free is only allowed",
		},
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
			name:          "readonly_discard_requires_as_bang",
			file:          "test_sources/compile_tests/readonly/discard_requires_unsafe.gecko",
			expectedError: "Unsafe Cast Required",
			expectedMsg:   "discarding readonly",
		},
		{
			name:          "as_bang_requires_unsafe",
			file:          "test_sources/compile_tests/readonly/as_bang_requires_unsafe.gecko",
			expectedError: "Unsafe Required",
			expectedMsg:   "as! is only allowed inside @unsafe",
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
			name:          "generic_inference_ambiguity",
			file:          "test_sources/compile_tests/type_checking/generic_inference_ambiguous.gecko",
			expectedError: "Type Inference Ambiguity",
			expectedMsg:   "Use explicit `<...>` type arguments",
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
		{
			name:          "intrinsic_requires_unsafe",
			file:          "test_sources/compile_tests/unsafe/intrinsic_requires_unsafe.gecko",
			expectedError: "Unsafe Required",
			expectedMsg:   "@write_volatile is only allowed",
		},
		{
			name:          "call_unsafe_requires_unsafe",
			file:          "test_sources/compile_tests/unsafe/call_unsafe_requires_unsafe.gecko",
			expectedError: "Unsafe Required",
			expectedMsg:   "call to @unsafe function",
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
