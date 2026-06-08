package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func hasAnySymbol(symbols string, names ...string) bool {
	for _, line := range strings.Split(symbols, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		symbol := fields[len(fields)-1]
		for _, name := range names {
			if symbol == name {
				return true
			}
		}
	}
	return false
}

func getProjectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "tests" {
		projectRoot = wd
	}
	return projectRoot
}

func buildFixtureBinary(t *testing.T, geckoPath, sourcePath, outputPath, backend string, extraArgs ...string) string {
	t.Helper()
	args := []string{"build", "--backend", backend, "--treeshake", "-o", outputPath, sourcePath}
	args = append(args, extraArgs...)
	cmd := exec.Command(geckoPath, args...)
	cmd.Dir = getProjectRoot(t)
	cmd.Env = append(os.Environ(), "GECKO_HOME="+getProjectRoot(t))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return string(out)
}

func readBinarySymbols(t *testing.T, binaryPath string) string {
	t.Helper()
	if _, err := exec.LookPath("nm"); err != nil {
		t.Skip("nm not available in test environment")
	}
	cmd := exec.Command("nm", binaryPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nm failed: %v\n%s", err, out)
	}
	return string(out)
}

func TestTreeshakeRemovesUnreachableInternalSymbol(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("treeshake linker GC flags are only asserted on darwin/linux")
	}

	geckoPath := buildGecko(t)
	projectRoot := getProjectRoot(t)
	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/treeshake/reachability/main.gecko")

	for _, backend := range allTestBackends {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_reachability_"+backend)
			_ = os.Remove(outputPath)

			buildFixtureBinary(t, geckoPath, sourcePath, outputPath, backend)
			symbols := readBinarySymbols(t, outputPath)

			if !hasAnySymbol(symbols, "symbols__ts_live", "_symbols__ts_live", "_ts_live", "ts_live") {
				t.Fatalf("expected reachable ts_live symbol in binary symbols:\n%s", symbols)
			}
			if hasAnySymbol(symbols, "symbols__ts_dead", "_symbols__ts_dead", "_ts_dead", "ts_dead") {
				t.Fatalf("expected unreachable ts_dead symbol to be removed by treeshake:\n%s", symbols)
			}
		})
	}
}

func TestTreeshakeKeepsExternalFunctionsAsRoots(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("treeshake linker GC flags are only asserted on darwin/linux")
	}

	geckoPath := buildGecko(t)
	projectRoot := getProjectRoot(t)
	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/treeshake/external_roots/main.gecko")

	for _, backend := range allTestBackends {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_external_roots_"+backend)
			_ = os.Remove(outputPath)

			buildFixtureBinary(t, geckoPath, sourcePath, outputPath, backend)
			symbols := readBinarySymbols(t, outputPath)

			if !hasAnySymbol(symbols, "api_used", "_api_used") {
				t.Fatalf("expected external root symbol api_used in binary:\n%s", symbols)
			}
			if !hasAnySymbol(symbols, "api_unused", "_api_unused") {
				t.Fatalf("expected external root symbol api_unused to be retained by anchor table:\n%s", symbols)
			}
		})
	}
}

func TestTreeshakeDynamicCallFallback(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("treeshake linker GC flags are only asserted on darwin/linux")
	}

	geckoPath := buildGecko(t)
	projectRoot := getProjectRoot(t)
	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/treeshake/dynamic_fallback/main.gecko")

	for _, backend := range allTestBackends {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_dynamic_fallback_"+backend)
			_ = os.Remove(outputPath)

			buildOutput := buildFixtureBinary(t, geckoPath, sourcePath, outputPath, backend)
			symbols := readBinarySymbols(t, outputPath)
			hasAutoDisableWarning := strings.Contains(buildOutput, "warning: treeshake disabled for this build due to dynamic-call patterns:")
			hasDynUnreachable := hasAnySymbol(symbols, "dynamicfallback__dyn_unreachable", "_dynamicfallback__dyn_unreachable", "_dyn_unreachable", "dyn_unreachable")

			if hasAutoDisableWarning {
				if !hasDynUnreachable {
					t.Fatalf("expected dyn_unreachable symbol to remain when treeshake auto-disables:\n%s", symbols)
				}
				return
			}

			// Backends that keep treeshake enabled are allowed to remove dyn_unreachable.
			if hasDynUnreachable {
				t.Logf("%s backend preserved dyn_unreachable without auto-disable warning", backend)
			}
		})
	}
}
