package tests

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

func requireTool(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s not available in test environment", tool)
	}
}

func writeLLVMFixtureMain(t *testing.T, returnCode int32) string {
	t.Helper()

	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "main.gecko")
	source := fmt.Sprintf(`package main

external func main(): int32 {
    let code: int32 = %d
    return code
}
`, returnCode)
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("failed writing fixture: %v", err)
	}
	return sourcePath
}

func runGeckoCommand(t *testing.T, geckoPath, projectRoot string, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command(geckoPath, args...)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GECKO_HOME="+projectRoot)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("command failed unexpectedly: %v\n%s", err, output)
		}
	}

	return string(output), exitCode
}

func newLLVMCompilerContext(t *testing.T, irOnly bool) *cli.Context {
	t.Helper()

	fs := flag.NewFlagSet("llvm-compiler-artifact", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("backend", "llvm", "")
	fs.String("target-arch", runtime.GOARCH, "")
	fs.String("target-platform", runtime.GOOS, "")
	fs.String("target-vendor", "", "")
	fs.Bool("print-ir", false, "")
	fs.Bool("ir-only", irOnly, "")
	llcArgs := cli.NewStringSlice()
	fs.Var(llcArgs, "llc-args", "")

	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse compiler context flags: %v", err)
	}

	return cli.NewContext(cli.NewApp(), fs, nil)
}

func diagnosticsString(diags []compiler.DiagnosticMessage) string {
	if len(diags) == 0 {
		return "<none>"
	}
	lines := make([]string, 0, len(diags))
	for _, diag := range diags {
		lines = append(lines, fmt.Sprintf("%s:%d:%d %s", "diag", diag.Line, diag.Column, diag.Message))
	}
	return strings.Join(lines, "\n")
}

func TestLLVMCompileCommandProducesObjectArtifact(t *testing.T) {
	requireTool(t, "llc")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)
	objectPath := sourcePath + ".o"

	_, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "compile", "--backend", "llvm", sourcePath)
	if exitCode != 0 {
		t.Fatalf("expected compile command to exit with code 0, got %d", exitCode)
	}
	if _, err := os.Stat(objectPath); err != nil {
		t.Fatalf("expected LLVM object artifact at %s: %v", objectPath, err)
	}
}

func TestLLVMOnlyExampleCompileCommandProducesObjectArtifact(t *testing.T) {
	requireTool(t, "llc")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := filepath.Join(projectRoot, "examples/llvm_only/main.gecko")
	objectPath := sourcePath + ".o"
	_ = os.Remove(objectPath)

	output, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "compile", sourcePath)
	if exitCode != 0 {
		t.Fatalf("expected compile command to exit with code 0, got %d\n%s", exitCode, output)
	}
	if _, err := os.Stat(objectPath); err != nil {
		t.Fatalf("expected LLVM object artifact at %s: %v", objectPath, err)
	}
}

func TestLLVMOnlyExampleRunCommandUsesSourceBackendAttribute(t *testing.T) {
	requireTool(t, "llc")
	requireTool(t, "clang")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 34)

	output, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "run", sourcePath)
	if exitCode != 34 {
		t.Fatalf("expected gecko run exit code 34 from source backend attribute fixture, got %d\n%s", exitCode, output)
	}
}

func TestLLVMCompileCommandIROnlyProducesIRArtifact(t *testing.T) {
	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)
	irPath := sourcePath + ".ll"

	_, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "compile", "--backend", "llvm", "--ir-only", sourcePath)
	if exitCode != 0 {
		t.Fatalf("expected ir-only compile command to exit with code 0, got %d", exitCode)
	}
	if _, err := os.Stat(irPath); err != nil {
		t.Fatalf("expected LLVM IR artifact at %s: %v", irPath, err)
	}
}

func TestLLVMCompileCommandSupportsInt32ReturnLiteral(t *testing.T) {
	requireTool(t, "llc")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)
	objectPath := sourcePath + ".o"

	// Re-write fixture to use a raw integer literal return to guard against i64 return emission.
	source := `package main
external func main(): int32 {
    return 0
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("failed writing fixture: %v", err)
	}

	output, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "compile", "--backend", "llvm", sourcePath)
	if exitCode != 0 {
		t.Fatalf("expected literal-return compile command to exit with code 0, got %d\n%s", exitCode, output)
	}
	if _, err := os.Stat(objectPath); err != nil {
		t.Fatalf("expected LLVM object artifact at %s: %v", objectPath, err)
	}
}

func TestLLVMBuildCommandProducesRunnableBinary(t *testing.T) {
	requireTool(t, "llc")
	requireTool(t, "clang")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)
	outputBin := filepath.Join(t.TempDir(), "llvm_build_output")

	output, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "build", "--backend", "llvm", "-o", outputBin, sourcePath)
	if exitCode != 0 {
		t.Fatalf("build command failed with exit %d\n%s", exitCode, output)
	}
	if _, err := os.Stat(outputBin); err != nil {
		t.Fatalf("expected built binary at %s: %v", outputBin, err)
	}

	runCmd := exec.Command(outputBin)
	runOutput, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected binary to exit with code 17, got success\n%s", runOutput)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("failed to execute built binary: %v\n%s", err, runOutput)
	}
	if exitErr.ExitCode() != 17 {
		t.Fatalf("expected built binary exit code 17, got %d\n%s", exitErr.ExitCode(), runOutput)
	}
}

func TestLLVMRunCommandExecutesBinary(t *testing.T) {
	requireTool(t, "llc")
	requireTool(t, "clang")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)

	output, exitCode := runGeckoCommand(t, geckoPath, projectRoot, "run", "--backend", "llvm", sourcePath)
	if exitCode != 17 {
		t.Fatalf("expected gecko run exit code 17, got %d\n%s", exitCode, output)
	}
}

func TestLLVMCompileHardFailsWithoutCFallback(t *testing.T) {
	requireTool(t, "llc")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)
	sourcePath := writeLLVMFixtureMain(t, 17)
	objectPath := sourcePath + ".o"
	cPath := sourcePath + ".c"

	output, _ := runGeckoCommand(
		t,
		geckoPath,
		projectRoot,
		"compile",
		"--backend",
		"llvm",
		"--llc-args=--definitely-invalid-llc-flag",
		sourcePath,
	)

	if !strings.Contains(output, "Compilation Backend Error") {
		t.Fatalf("expected LLVM compile hard-fail output, got:\n%s", output)
	}
	if _, err := os.Stat(objectPath); err == nil {
		t.Fatalf("did not expect object artifact on LLVM failure: %s", objectPath)
	}
	if _, err := os.Stat(cPath); err == nil {
		t.Fatalf("did not expect C fallback artifact on LLVM failure: %s", cPath)
	}
}

func TestLLVMCompileReportsExpressionErrorsWithoutPanic(t *testing.T) {
	requireTool(t, "llc")

	geckoPath := buildGecko(t)
	projectRoot := projectRootForTests(t)

	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "panic_guard.gecko")
	source := `package main
declare external variardic func printf(format: string): int32

external func main(): int32 {
    let x: int32 = 1
    printf("%d\n", missing + x)
    return 0
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("failed writing fixture: %v", err)
	}

	output, _ := runGeckoCommand(t, geckoPath, projectRoot, "compile", "--backend", "llvm", sourcePath)
	if strings.Contains(output, "panic:") {
		t.Fatalf("llvm compile should not panic on invalid expression\n%s", output)
	}
	if !outputHasErrorSummary(output) {
		t.Fatalf("expected llvm compile to report an error summary for invalid expression\n%s", output)
	}
}

func TestCompilerLLVMArtifactContract(t *testing.T) {
	requireTool(t, "llc")

	sourcePath := writeLLVMFixtureMain(t, 17)
	baseCfg := config.CompileCfg{
		Arch:      runtime.GOARCH,
		Platform:  runtime.GOOS,
		Vendor:    "",
		TargetKey: "",
		Treeshake: true,
		CFlags:    []string{},
		CLFlags:   []string{},
		CObjects:  []string{},
		Project:   nil,
	}

	compiler.ResetCompilationState()
	irCfg := baseCfg
	irCfg.Ctx = newLLVMCompilerContext(t, true)
	irArtifact := compiler.Compile(sourcePath, &irCfg)
	if irArtifact == "" {
		t.Fatalf("expected LLVM ir-only compile to return .ll artifact, got empty path\nDiagnostics:\n%s", diagnosticsString(compiler.GetAllErrors()))
	}
	if filepath.Ext(irArtifact) != ".ll" {
		t.Fatalf("expected LLVM ir-only artifact extension .ll, got %s", irArtifact)
	}
	if _, err := os.Stat(irArtifact); err != nil {
		t.Fatalf("expected LLVM ir-only artifact to exist at %s: %v", irArtifact, err)
	}

	compiler.ResetCompilationState()
	objCfg := baseCfg
	objCfg.Ctx = newLLVMCompilerContext(t, false)
	objArtifact := compiler.Compile(sourcePath, &objCfg)
	if objArtifact == "" {
		t.Fatalf("expected LLVM object compile to return .o artifact, got empty path\nDiagnostics:\n%s", diagnosticsString(compiler.GetAllErrors()))
	}
	if filepath.Ext(objArtifact) != ".o" {
		t.Fatalf("expected LLVM object artifact extension .o, got %s", objArtifact)
	}
	if _, err := os.Stat(objArtifact); err != nil {
		t.Fatalf("expected LLVM object artifact to exist at %s: %v", objArtifact, err)
	}
}
