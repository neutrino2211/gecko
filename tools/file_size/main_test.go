package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSizePolicy(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "init", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initializing fixture repository: %v\n%s", err, output)
	}
	write := func(path, content string) {
		t.Helper()
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.go", strings.Repeat("line\n", 500))
	if err := checkRoot(root); err != nil {
		t.Fatalf("500-line source should pass: %v", err)
	}
	write("source.go", strings.Repeat("line\n", 500)+"last line")
	if err := checkRoot(root); err == nil || !strings.Contains(err.Error(), "source.go: 501 lines") {
		t.Fatalf("expected rejection of final line without newline, got %v", err)
	}
	write("source.go", strings.Repeat("line\r\n", 500))
	write("LICENSE", strings.Repeat("license\n", 600))
	write("package-lock.json", strings.Repeat("generated\n", 600))
	write("examples/GNote/web/notes.html", strings.Repeat("generated\n", 600))
	write("asset.bin", "\x00"+strings.Repeat("binary\n", 600))
	write(".gitignore", "ignored.txt\n")
	write("ignored.txt", strings.Repeat("ignored\n", 600))
	if err := checkRoot(root); err != nil {
		t.Fatalf("CRLF files and approved exemptions should pass: %v", err)
	}
	write("guide.md", strings.Repeat("documentation\n", 501))
	if err := checkRoot(root); err == nil || !strings.Contains(err.Error(), "guide.md: 501 lines") {
		t.Fatalf("expected documentation limit, got %v", err)
	}
}
