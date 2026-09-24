package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryRuntimeModes(t *testing.T) {
	binary := buildGecko(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fixture string
		flags         []string
		exit          int
	}{
		{"manual_cleanup", "memory_automation", []string{"--no-auto-drop"}, 3},
		{"borrow_conflict", "borrow_conflict", nil, 255},
		{"borrow_writer_conflict", "borrow_writer_conflict", nil, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := filepath.Join(root, "test_sources", "compile_tests", tc.fixture, "main.gecko")
			args := append([]string{"run"}, tc.flags...)
			args = append(args, source)
			cmd := exec.Command(binary, args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "GECKO_HOME="+root)
			output, runErr := cmd.CombinedOutput()
			text := string(output)
			assertNoBackendPanic(t, "c", text)
			if outputHasCompileErrors(text) || strings.Contains(text, "error:") {
				t.Fatalf("compile failed: %s", text)
			}
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				t.Fatalf("expected exit %d, got %v: %s", tc.exit, runErr, text)
			}
			if exitErr.ExitCode() != tc.exit {
				t.Fatalf("exit %d, want %d: %s", exitErr.ExitCode(), tc.exit, text)
			}
		})
	}
}

func TestPrintExpandedOwnership(t *testing.T) {
	binary := buildGecko(t)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "test_sources", "compile_tests", "owned_aggregates", "main.gecko")
	cmd := exec.Command(binary, "compile", "--ir-only", "--print-expanded", source)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GECKO_HOME="+root)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile failed: %v\n%s", err, output)
	}
	for _, fragment := range []string{"__owned_", "__moved_value", "__moved_arg", "Drop__drop", ".active ="} {
		if !strings.Contains(string(output), fragment) {
			t.Fatalf("expanded output lacks %q", fragment)
		}
	}
}
