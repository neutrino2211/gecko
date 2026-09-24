package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxLines = 500

var generatedFiles = map[string]bool{
	"examples/GNote/web/notes.html":       true,
	"examples/llvm_kernel/kernel.ll":      true,
	"examples/llvm_kernel/shell.gecko.ll": true,
}

func exempt(path string) bool {
	if generatedFiles[path] {
		return true
	}
	switch filepath.Base(path) {
	case "go.sum", "package-lock.json", "pnpm-lock.yaml", "yarn.lock":
		return true
	}
	name := strings.ToUpper(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	return name == "LICENSE" || name == "COPYING" || name == "NOTICE"
}

func check() error {
	rootOutput, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}
	root := strings.TrimSpace(string(rootOutput))
	return checkRoot(root)
}

func checkRoot(root string) error {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("listing maintained files: %w", err)
	}
	seen := make(map[string]bool)
	var violations []string
	checked := 0
	for _, path := range strings.Split(string(output), "\x00") {
		if path == "" || seen[path] || exempt(path) {
			continue
		}
		seen[path] = true
		fullPath := filepath.Join(root, path)
		info, err := os.Lstat(fullPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspecting %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
			continue
		}
		checked++
		lines := bytes.Count(content, []byte{'\n'})
		if len(content) > 0 && content[len(content)-1] != '\n' {
			lines++
		}
		if lines > maxLines {
			violations = append(violations, fmt.Sprintf("%s: %d lines", path, lines))
		}
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		return fmt.Errorf("files exceed %d lines:\n%s", maxLines, strings.Join(violations, "\n"))
	}
	fmt.Printf("Checked %d maintained text files: all at or below %d lines.\n", checked, maxLines)
	return nil
}

func main() {
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
