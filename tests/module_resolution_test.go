// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func newTestCLIContext(t *testing.T, backend string) *cli.Context {
	t.Helper()

	app := cli.NewApp()
	fs := flag.NewFlagSet("module-resolution-test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("backend", backend, "")
	fs.String("target-arch", runtime.GOARCH, "")
	fs.String("target-platform", runtime.GOOS, "")
	fs.String("target-vendor", "", "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("failed to build cli context: %v", err)
	}

	return cli.NewContext(app, fs, nil)
}

func compileAndCollectErrors(t *testing.T, sourcePath string, projectCfg *config.ProjectConfig, backend string) []compiler.DiagnosticMessage {
	t.Helper()

	compiler.ResetCompilationState()
	compiler.Compile(sourcePath, &config.CompileCfg{
		Arch:      runtime.GOARCH,
		Platform:  runtime.GOOS,
		Vendor:    "",
		TargetKey: "",
		CFlags:    []string{},
		CLFlags:   []string{},
		CObjects:  []string{},
		CheckOnly: true,
		Ctx:       newTestCLIContext(t, backend),
		Project:   projectCfg,
	})

	return compiler.GetAllErrors()
}

func formatCompileErrors(errs []compiler.DiagnosticMessage) string {
	if len(errs) == 0 {
		return ""
	}

	lines := make([]string, 0, len(errs))
	for _, err := range errs {
		lines = append(lines, fmt.Sprintf("%s (%d:%d)", err.Message, err.Line, err.Column))
	}

	return strings.Join(lines, "\n")
}

func runResolutionCompileChecks(t *testing.T, sourcePath string, projectCfg *config.ProjectConfig, assertFn func(t *testing.T, backend string, errs []compiler.DiagnosticMessage)) {
	t.Helper()

	for _, backend := range allTestBackends {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			errs := compileAndCollectErrors(t, sourcePath, projectCfg, backend)
			assertFn(t, backend, errs)
		})
	}
}

func TestModuleResolutionPrefersImporterDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gecko_module_resolution_relative")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	writeFile(t, filepath.Join(projectDir, "gecko.toml"), `
[package]
name = "resolver-test"
version = "0.1.0"

[build.entries]
main = "src/main.gecko"
`)

	// Import target beside importer (preferred over project-root fallback).
	writeFile(t, filepath.Join(projectDir, "src", "local.gecko"), `package local
public class SrcLocalType {
    let value: int32
}
`)

	// Same module name at project root; picking this should fail this test.
	writeFile(t, filepath.Join(projectDir, "local.gecko"), `package local
public class RootLocalType {
    let value: int32
}
`)

	mainPath := filepath.Join(projectDir, "src", "main.gecko")
	writeFile(t, mainPath, `package main
import local use { SrcLocalType }

external func main(): int32 {
    let v: SrcLocalType = SrcLocalType { value: 7 }
    return v.value
}
`)

	projectCfg, err := config.LoadProjectConfig(projectDir)
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}

	runResolutionCompileChecks(t, mainPath, projectCfg, func(t *testing.T, backend string, errs []compiler.DiagnosticMessage) {
		if len(errs) > 0 {
			t.Fatalf("expected compile success with importer-relative resolution, got errors:\n%s", formatCompileErrors(errs))
		}
	})
}

func TestModuleResolutionFallsBackToVendor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gecko_module_resolution_vendor")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	writeFile(t, filepath.Join(projectDir, "gecko.toml"), `
[package]
name = "resolver-test"
version = "0.1.0"

[build.entries]
main = "main.gecko"
`)

	writeFile(t, filepath.Join(projectDir, "vendor", "vendored.gecko"), `package vendored
public class VendoredType {
    let value: int32
}
`)

	mainPath := filepath.Join(projectDir, "main.gecko")
	writeFile(t, mainPath, `package main
import vendored use { VendoredType }

external func main(): int32 {
    let v: VendoredType = VendoredType { value: 9 }
    return v.value
}
`)

	projectCfg, err := config.LoadProjectConfig(projectDir)
	if err != nil {
		t.Fatalf("failed to load project config: %v", err)
	}

	runResolutionCompileChecks(t, mainPath, projectCfg, func(t *testing.T, backend string, errs []compiler.DiagnosticMessage) {
		if len(errs) > 0 {
			t.Fatalf("expected vendor fallback to resolve import, got errors:\n%s", formatCompileErrors(errs))
		}
	})
}

func TestStdlibImportStripsPrefixDuringResolution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gecko_module_resolution_std")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	geckoHome := filepath.Join(tmpDir, "gecko_home")
	projectDir := filepath.Join(tmpDir, "project")

	writeFile(t, filepath.Join(geckoHome, "stdlib", "option.gecko"), `package option
public class Option {
    let has_value: bool
}
`)

	mainPath := filepath.Join(projectDir, "main.gecko")
	writeFile(t, mainPath, `package main
import std.option use { Option }

external func main(): int32 {
    let opt: Option = Option { has_value: true }
    if (opt.has_value) {
        return 1
    }
    return 0
}
`)

	oldHome := os.Getenv("GECKO_HOME")
	if err := os.Setenv("GECKO_HOME", geckoHome); err != nil {
		t.Fatalf("failed to set GECKO_HOME: %v", err)
	}
	defer os.Setenv("GECKO_HOME", oldHome)

	runResolutionCompileChecks(t, mainPath, nil, func(t *testing.T, backend string, errs []compiler.DiagnosticMessage) {
		if len(errs) > 0 {
			t.Fatalf("expected std.option to resolve to $GECKO_HOME/stdlib/option.gecko, got errors:\n%s", formatCompileErrors(errs))
		}
	})
}

func TestDirectoryImportResolvesQualifiedTypeForDottedImportPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gecko_module_resolution_dotted_dir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	writeFile(t, filepath.Join(projectDir, "mylib", "shapes", "circle.gecko"), `package shapes
public class Circle {
    let radius: int32
}

impl Circle {
    public func new(radius: int32): Circle {
        return Circle { radius: radius }
    }

    public func area(self): int32 {
        return self.radius * self.radius
    }
}
`)

	mainPath := filepath.Join(projectDir, "main.gecko")
	writeFile(t, mainPath, `package main
import mylib.shapes

external func main(): int32 {
    let c: shapes.Circle = shapes.Circle::new(4)
    return c.area()
}
`)

	runResolutionCompileChecks(t, mainPath, nil, func(t *testing.T, backend string, errs []compiler.DiagnosticMessage) {
		if len(errs) > 0 {
			t.Fatalf("expected dotted directory import to resolve qualified type, got errors:\n%s", formatCompileErrors(errs))
		}
	})
}

func TestSequentialCompilesDoNotLeakImportCacheAcrossFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gecko_module_resolution_cache")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectA := filepath.Join(tmpDir, "project_a")
	projectB := filepath.Join(tmpDir, "project_b")

	mainA := filepath.Join(projectA, "main.gecko")
	writeFile(t, filepath.Join(projectA, "shared.gecko"), `package shared
public class AOnly {
    let value: int32
}
`)
	writeFile(t, mainA, `package main
import shared use { AOnly }

external func main(): int32 {
    let v: AOnly = AOnly { value: 1 }
    return v.value
}
`)

	mainB := filepath.Join(projectB, "main.gecko")
	writeFile(t, filepath.Join(projectB, "shared.gecko"), `package shared
public class BOnly {
    let value: int32
}
`)
	writeFile(t, mainB, `package main
import shared use { BOnly }

external func main(): int32 {
    let v: BOnly = BOnly { value: 2 }
    return v.value
}
`)

	for _, backend := range allTestBackends {
		backend := backend
		t.Run(backend, func(t *testing.T) {
			errsA := compileAndCollectErrors(t, mainA, nil, backend)
			if len(errsA) > 0 {
				t.Fatalf("first compile unexpectedly failed:\n%s", formatCompileErrors(errsA))
			}

			errsB := compileAndCollectErrors(t, mainB, nil, backend)
			if len(errsB) > 0 {
				t.Fatalf("second compile unexpectedly failed (possible cross-file import cache leak):\n%s", formatCompileErrors(errsB))
			}
		})
	}
}
