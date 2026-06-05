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
	t.Helper()

	projectRoot := projectRootForTests(t)
	sourcePath := filepath.Join(projectRoot, sourceFile)

	cmd := exec.Command(geckoPath, "compile", "--backend", "llvm", "--ir-only", sourcePath)
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

func TestLLVMParityUnsupportedFeatures(t *testing.T) {
	geckoPath := buildGecko(t)

	tests := []llvmUnsupportedCase{
		{
			name:            "imports",
			file:            "test_sources/compile_tests/imports/main.gecko",
			mustContain:     []string{"Unsupported Feature", "Feature 'imports' is not supported by the 'llvm' backend"},
			requireErrorSum: true,
		},
		{
			name:            "volatile",
			file:            "test_sources/compile_tests/volatile/volatile_pointer.gecko",
			mustContain:     []string{"Unsupported Feature", "Feature 'volatile' is not supported by the 'llvm' backend"},
			requireErrorSum: true,
		},
		{
			name:            "error_handling_try_invalid",
			file:            "test_sources/compile_tests/error_handling_try_invalid/main.gecko",
			mustContain:     []string{"Unsupported Feature"},
			mustContainAny:  []string{"Feature 'impl' is not supported by the 'llvm' backend", "Feature 'generics' is not supported by the 'llvm' backend"},
			requireErrorSum: true,
		},
	}

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
