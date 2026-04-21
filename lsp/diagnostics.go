package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"
	"go.lsp.dev/protocol"
)

func uriToPath(uri string) string {
	path := string(uri)
	if strings.HasPrefix(path, "file://") {
		path = path[7:]
	}
	return path
}

func pathToURI(path string) protocol.DocumentURI {
	if !strings.HasPrefix(path, "file://") {
		path = "file://" + path
	}
	return protocol.DocumentURI(path)
}

func RunCompilerCheck(uri string, content string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	filePath := uriToPath(uri)

	// Write content to a temp file for compilation
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "gecko-lsp-check-"+filepath.Base(filePath))

	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		log.Printf("Failed to write temp file: %v", err)
		return diagnostics, err
	}
	defer os.Remove(tempFile)

	// Create a CLI context with backend flag set
	app := &cli.App{}
	flagSet := flag.NewFlagSet("lsp", flag.ContinueOnError)
	flagSet.String("backend", "c", "")
	ctx := cli.NewContext(app, flagSet, nil)

	// Run compiler in check-only mode
	compiler.ResetErrorScopes()
	compiler.Compile(tempFile, &config.CompileCfg{
		Arch:      "amd64",
		Platform:  "darwin",
		CheckOnly: true,
		Ctx:       ctx,
	})

	// Extract errors from compiler
	for _, err := range compiler.GetAllErrors() {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(err.Line - 1),
					Character: uint32(err.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(err.Line - 1),
					Character: uint32(err.Column + 10),
				},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "gecko",
			Message:  err.Message,
		})
	}

	// Extract warnings from compiler
	for _, warn := range compiler.GetAllWarnings() {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(warn.Line - 1),
					Character: uint32(warn.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(warn.Line - 1),
					Character: uint32(warn.Column + 10),
				},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "gecko",
			Message:  warn.Message,
		})
	}

	return diagnostics, nil
}
