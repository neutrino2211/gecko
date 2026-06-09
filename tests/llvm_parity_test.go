// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type llvmParityCase struct {
	name string
	file string
}

type llvmUnsupportedCase struct {
	name            string
	file            string
	mustContain     []string
	mustContainAny  []string
	requireErrorSum bool
}

func projectRootForTests(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if filepath.Base(wd) == "tests" {
		return filepath.Dir(wd)
	}
	return wd
}

func runLLVMIROnlyCompile(t *testing.T, geckoPath, sourceFile string) (string, int) {
	return runIROnlyCompileForBackend(t, geckoPath, "llvm", sourceFile)
}

func runIROnlyCompileForBackend(t *testing.T, geckoPath, backend, sourceFile string) (string, int) {
	t.Helper()

	projectRoot := projectRootForTests(t)
	sourcePath := filepath.Join(projectRoot, sourceFile)

	cmd := exec.Command(geckoPath, "compile", "--backend", backend, "--ir-only", sourcePath)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("LLVM compile command failed unexpectedly for %s: %v", sourceFile, err)
		}
	}

	return string(output), exitCode
}

type parityExpectation struct {
	name string
	file string
}

func outputExcerpt(output string) string {
	const maxChars = 2000
	if len(output) <= maxChars {
		return output
	}
	return output[:maxChars] + "\n...<truncated>...\n"
}

func outputHasErrorSummary(output string) bool {
	matches := errorsGeneratedPattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		errorCount, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			continue
		}
		if errorCount > 0 {
			return true
		}
	}
	return false
}

func TestLLVMParityPanicSafety(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmParityCase{
		{name: "loops_break_continue", file: "test_sources/compile_tests/loops/break_continue.gecko"},
		{name: "out_params", file: "test_sources/compile_tests/out_params/main.gecko"},
		{name: "intrinsics_basic", file: "test_sources/compile_tests/intrinsics/basic.gecko"},
		{name: "logical_ops", file: "test_sources/compile_tests/logical_ops/main.gecko"},
		{name: "type_inference", file: "test_sources/compile_tests/type_inference/main.gecko"},
		{name: "struct_literal", file: "test_sources/compile_tests/struct_literal/struct_literal.gecko"},
		{name: "llvm_parity_enum_slice", file: "test_sources/compile_tests/llvm_parity/enum_slice.gecko"},
		{name: "llvm_parity_method_chain_slice", file: "test_sources/compile_tests/llvm_parity/method_chain_slice.gecko"},
		{name: "llvm_parity_inherent_impl_chain", file: "test_sources/compile_tests/llvm_parity/inherent_impl_chain/main.gecko"},
		{name: "llvm_parity_trait_impl_call", file: "test_sources/compile_tests/llvm_parity/trait_impl_call/main.gecko"},
		{name: "llvm_parity_trait_inheritance_override", file: "test_sources/compile_tests/llvm_parity/trait_inheritance_override/main.gecko"},
		{name: "llvm_parity_opaque_external_decl", file: "test_sources/compile_tests/llvm_parity/opaque_external_decl/main.gecko"},
		{name: "llvm_parity_intrinsic_is_null", file: "test_sources/compile_tests/llvm_parity/intrinsic_is_null/main.gecko"},
		{name: "llvm_parity_generic_option_try", file: "test_sources/compile_tests/error_handling_try_invalid/main.gecko"},
		{name: "llvm_parity_unresolved_trait_method", file: "test_sources/compile_tests/llvm_parity/unresolved_trait_method/main.gecko"},
		{name: "llvm_parity_visibility_denied_method", file: "test_sources/compile_tests/llvm_parity/visibility_denied_method/main.gecko"},
		{name: "llvm_parity_unresolved_foreign_module_method", file: "test_sources/compile_tests/llvm_parity/unresolved_foreign_module_method/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestLLVMParityFirstSlice(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmParityCase{
		{name: "imports", file: "test_sources/compile_tests/imports/main.gecko"},
		{name: "type_inference", file: "test_sources/compile_tests/llvm_parity/type_inference_slice.gecko"},
		{name: "loops_break_continue", file: "test_sources/compile_tests/loops/break_continue.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM parity slice fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if strings.Contains(output, "Unsupported Feature") {
				t.Fatalf("LLVM parity slice fixture must not hit unsupported feature gate for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if outputHasErrorSummary(output) {
				t.Fatalf("LLVM parity slice fixture must compile without reported errors for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestLLVMParityEmitterGapSlices(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmParityCase{
		{name: "enum_slice", file: "test_sources/compile_tests/llvm_parity/enum_slice.gecko"},
		{name: "method_chain_slice", file: "test_sources/compile_tests/llvm_parity/method_chain_slice.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM emitter-gap fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if strings.Contains(output, "Unsupported Feature") {
				t.Fatalf("LLVM emitter-gap fixture must not hit unsupported feature gate for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if outputHasErrorSummary(output) {
				t.Fatalf("LLVM emitter-gap fixture must compile without reported errors for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestLLVMParityTraitImplLoweringSlices(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmParityCase{
		{name: "inherent_impl_chain", file: "test_sources/compile_tests/llvm_parity/inherent_impl_chain/main.gecko"},
		{name: "trait_impl_call", file: "test_sources/compile_tests/llvm_parity/trait_impl_call/main.gecko"},
		{name: "trait_inheritance_override", file: "test_sources/compile_tests/llvm_parity/trait_inheritance_override/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM trait/impl parity fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if strings.Contains(output, "Unsupported Feature") {
				t.Fatalf("LLVM trait/impl parity fixture must not hit unsupported feature gate for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if outputHasErrorSummary(output) {
				t.Fatalf("LLVM trait/impl parity fixture must compile without reported errors for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestLLVMParityGNotePrereqSlices(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmParityCase{
		{name: "opaque_external_decl", file: "test_sources/compile_tests/llvm_parity/opaque_external_decl/main.gecko"},
		{name: "intrinsic_is_null", file: "test_sources/compile_tests/llvm_parity/intrinsic_is_null/main.gecko"},
		{name: "generic_option_try", file: "test_sources/compile_tests/error_handling_try_invalid/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM GNote-prereq fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if strings.Contains(output, "Unsupported Feature") {
				t.Fatalf("LLVM GNote-prereq fixture must not hit unsupported feature gate for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
			if outputHasErrorSummary(output) {
				t.Fatalf("LLVM GNote-prereq fixture must compile without reported errors for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestBackendParityFirstSliceCompileBehavior(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []parityExpectation{
		{name: "imports", file: "test_sources/compile_tests/imports/main.gecko"},
		{name: "type_inference", file: "test_sources/compile_tests/llvm_parity/type_inference_slice.gecko"},
		{name: "loops_break_continue", file: "test_sources/compile_tests/loops/break_continue.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "c", tc.file)
			llvmOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "llvm", tc.file)

			if strings.Contains(cOutput, "panic:") {
				t.Fatalf("C backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(cOutput))
			}
			if strings.Contains(llvmOutput, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(llvmOutput))
			}

			cHasErrors := outputHasErrorSummary(cOutput)
			llvmHasErrors := outputHasErrorSummary(llvmOutput)
			if cHasErrors != llvmHasErrors {
				t.Fatalf(
					"Compile behavior mismatch for %s: C errors=%v, LLVM errors=%v\nC output:\n%s\nLLVM output:\n%s",
					tc.file, cHasErrors, llvmHasErrors, outputExcerpt(cOutput), outputExcerpt(llvmOutput),
				)
			}
		})
	}
}

func TestBackendParityGNotePrereqCompileBehavior(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []parityExpectation{
		{name: "opaque_external_decl", file: "test_sources/compile_tests/llvm_parity/opaque_external_decl/main.gecko"},
		{name: "intrinsic_is_null", file: "test_sources/compile_tests/llvm_parity/intrinsic_is_null/main.gecko"},
		{name: "generic_option_try", file: "test_sources/compile_tests/error_handling_try_invalid/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "c", tc.file)
			llvmOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "llvm", tc.file)

			if strings.Contains(cOutput, "panic:") {
				t.Fatalf("C backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(cOutput))
			}
			if strings.Contains(llvmOutput, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(llvmOutput))
			}

			cHasErrors := outputHasErrorSummary(cOutput)
			llvmHasErrors := outputHasErrorSummary(llvmOutput)
			if cHasErrors != llvmHasErrors {
				t.Fatalf(
					"Compile behavior mismatch for %s: C errors=%v, LLVM errors=%v\nC output:\n%s\nLLVM output:\n%s",
					tc.file, cHasErrors, llvmHasErrors, outputExcerpt(cOutput), outputExcerpt(llvmOutput),
				)
			}
		})
	}
}

func TestBackendParityTraitImplLoweringCompileBehavior(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []parityExpectation{
		{name: "inherent_impl_chain", file: "test_sources/compile_tests/llvm_parity/inherent_impl_chain/main.gecko"},
		{name: "trait_impl_call", file: "test_sources/compile_tests/llvm_parity/trait_impl_call/main.gecko"},
		{name: "trait_inheritance_override", file: "test_sources/compile_tests/llvm_parity/trait_inheritance_override/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "c", tc.file)
			llvmOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "llvm", tc.file)

			if strings.Contains(cOutput, "panic:") {
				t.Fatalf("C backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(cOutput))
			}
			if strings.Contains(llvmOutput, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(llvmOutput))
			}

			cHasErrors := outputHasErrorSummary(cOutput)
			llvmHasErrors := outputHasErrorSummary(llvmOutput)
			if cHasErrors != llvmHasErrors {
				t.Fatalf(
					"Compile behavior mismatch for %s: C errors=%v, LLVM errors=%v\nC output:\n%s\nLLVM output:\n%s",
					tc.file, cHasErrors, llvmHasErrors, outputExcerpt(cOutput), outputExcerpt(llvmOutput),
				)
			}
		})
	}
}

func TestBackendParityEmitterGapSlicesCompileBehavior(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []parityExpectation{
		{name: "enum_slice", file: "test_sources/compile_tests/llvm_parity/enum_slice.gecko"},
		{name: "method_chain_slice", file: "test_sources/compile_tests/llvm_parity/method_chain_slice.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "c", tc.file)
			llvmOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "llvm", tc.file)

			if strings.Contains(cOutput, "panic:") {
				t.Fatalf("C backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(cOutput))
			}
			if strings.Contains(llvmOutput, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(llvmOutput))
			}

			cHasErrors := outputHasErrorSummary(cOutput)
			llvmHasErrors := outputHasErrorSummary(llvmOutput)
			if cHasErrors != llvmHasErrors {
				t.Fatalf(
					"Compile behavior mismatch for %s: C errors=%v, LLVM errors=%v\nC output:\n%s\nLLVM output:\n%s",
					tc.file, cHasErrors, llvmHasErrors, outputExcerpt(cOutput), outputExcerpt(llvmOutput),
				)
			}
		})
	}
}

func TestLLVMParityTraitImplDiagnostics(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmUnsupportedCase{
		{
			name:            "unresolved_trait_method",
			file:            "test_sources/compile_tests/llvm_parity/unresolved_trait_method/main.gecko",
			mustContain:     []string{"Function resolution error", `Trait method "greet" on type "Person" has no lowered implementation in LLVM`},
			requireErrorSum: true,
		},
		{
			name:            "visibility_denied_method",
			file:            "test_sources/compile_tests/llvm_parity/visibility_denied_method/main.gecko",
			mustContain:     []string{"Visibility Error", "method 'hidden' is private (default) and can only be accessed within the same file"},
			requireErrorSum: true,
		},
		{
			name:            "unresolved_foreign_module_method",
			file:            "test_sources/compile_tests/llvm_parity/unresolved_foreign_module_method/main.gecko",
			mustContain:     []string{"Resolution Error", "Could not resolve module 'missing' while resolving receiver type 'missing.Payload' for method 'size'"},
			requireErrorSum: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("LLVM diagnostic fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}

			for _, want := range tc.mustContain {
				if !strings.Contains(output, want) {
					t.Fatalf("Expected output for %s to contain %q\nOutput:\n%s", tc.file, want, outputExcerpt(output))
				}
			}

			if tc.requireErrorSum && !outputHasErrorSummary(output) {
				t.Fatalf("Expected LLVM diagnostic fixture %s to report non-zero compile errors\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}

func TestBackendParityTraitImplDiagnosticsCompileBehavior(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []parityExpectation{
		{name: "unresolved_trait_method", file: "test_sources/compile_tests/llvm_parity/unresolved_trait_method/main.gecko"},
		{name: "visibility_denied_method", file: "test_sources/compile_tests/llvm_parity/visibility_denied_method/main.gecko"},
		{name: "unresolved_foreign_module_method", file: "test_sources/compile_tests/llvm_parity/unresolved_foreign_module_method/main.gecko"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "c", tc.file)
			llvmOutput, _ := runIROnlyCompileForBackend(t, geckoPath, "llvm", tc.file)

			if strings.Contains(cOutput, "panic:") {
				t.Fatalf("C backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(cOutput))
			}
			if strings.Contains(llvmOutput, "panic:") {
				t.Fatalf("LLVM backend must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(llvmOutput))
			}

			cHasErrors := outputHasErrorSummary(cOutput)
			llvmHasErrors := outputHasErrorSummary(llvmOutput)
			if cHasErrors != llvmHasErrors {
				t.Fatalf(
					"Compile behavior mismatch for %s: C errors=%v, LLVM errors=%v\nC output:\n%s\nLLVM output:\n%s",
					tc.file, cHasErrors, llvmHasErrors, outputExcerpt(cOutput), outputExcerpt(llvmOutput),
				)
			}
		})
	}
}

func TestLLVMParityUnsupportedFeatures(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmUnsupportedCase{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := runLLVMIROnlyCompile(t, geckoPath, tc.file)

			if strings.Contains(output, "panic:") {
				t.Fatalf("Unsupported-feature fixture must not panic for %s\nOutput:\n%s", tc.file, outputExcerpt(output))
			}

			for _, want := range tc.mustContain {
				if !strings.Contains(output, want) {
					t.Fatalf("Expected output for %s to contain %q\nOutput:\n%s", tc.file, want, outputExcerpt(output))
				}
			}

			if len(tc.mustContainAny) > 0 {
				found := false
				for _, want := range tc.mustContainAny {
					if strings.Contains(output, want) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected output for %s to contain at least one of %v\nOutput:\n%s", tc.file, tc.mustContainAny, outputExcerpt(output))
				}
			}

			if tc.requireErrorSum && !outputHasErrorSummary(output) {
				t.Fatalf("Expected LLVM unsupported fixture %s to report non-zero compile errors\nOutput:\n%s", tc.file, outputExcerpt(output))
			}
		})
	}
}
