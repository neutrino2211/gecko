package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

func buildFixtureBinary(t *testing.T, geckoPath, sourcePath, outputPath string, extraArgs ...string) string {
	t.Helper()
	args := []string{"build", "--backend", "c", "--treeshake", "-o", outputPath, sourcePath}
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
	outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_reachability")
	_ = os.Remove(outputPath)

	buildFixtureBinary(t, geckoPath, sourcePath, outputPath)
	symbols := readBinarySymbols(t, outputPath)

	if !strings.Contains(symbols, "symbols__ts_live") {
		t.Fatalf("expected reachable symbol symbols__ts_live in binary symbols:\n%s", symbols)
	}
	if strings.Contains(symbols, "symbols__ts_dead") {
		t.Fatalf("expected unreachable symbol symbols__ts_dead to be removed by treeshake:\n%s", symbols)
	}
}

func TestTreeshakeKeepsExternalFunctionsAsRoots(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("treeshake linker GC flags are only asserted on darwin/linux")
	}

	geckoPath := buildGecko(t)
	projectRoot := getProjectRoot(t)

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/treeshake/external_roots/main.gecko")
	outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_external_roots")
	_ = os.Remove(outputPath)

	buildFixtureBinary(t, geckoPath, sourcePath, outputPath)
	symbols := readBinarySymbols(t, outputPath)

	if !strings.Contains(symbols, "api_used") {
		t.Fatalf("expected external root symbol api_used in binary:\n%s", symbols)
	}
	if !strings.Contains(symbols, "api_unused") {
		t.Fatalf("expected external root symbol api_unused to be retained by anchor table:\n%s", symbols)
	}
}

func TestTreeshakeDynamicCallFallback(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("treeshake linker GC flags are only asserted on darwin/linux")
	}

	geckoPath := buildGecko(t)
	projectRoot := getProjectRoot(t)

	sourcePath := filepath.Join(projectRoot, "test_sources/compile_tests/treeshake/dynamic_fallback/main.gecko")
	outputPath := filepath.Join(os.TempDir(), "gecko_treeshake_dynamic_fallback")
	_ = os.Remove(outputPath)

	buildOutput := buildFixtureBinary(t, geckoPath, sourcePath, outputPath)
	if !strings.Contains(buildOutput, "warning: treeshake disabled for this build due to dynamic-call patterns:") {
		t.Fatalf("expected treeshake auto-disable warning, got:\n%s", buildOutput)
	}

	symbols := readBinarySymbols(t, outputPath)
	if !strings.Contains(symbols, "dynamicfallback__dyn_unreachable") {
		t.Fatalf("expected dyn_unreachable symbol to remain when treeshake auto-disables:\n%s", symbols)
	}
}
