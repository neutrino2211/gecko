package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/frontend"
	"go.lsp.dev/protocol"
)

func TestCompilerDiagnosticsReuseEditorFrontend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	source := "package main\ndeclare external func puts(text: string): int32\nfunc main(): int32 { return 0 }\n"
	session := frontend.NewSession()
	ctx, err := analysis.NewAnalysisContextWithSession(path, source, os.ReadFile, session)
	if err != nil {
		t.Fatal(err)
	}
	beforeParses, beforeAnalyses := session.Counts()
	diagnostics, err := runWorkspaceCheckWithFrontend(string(pathToURI(path)), source, nil, "", ctx.Frontend, session)
	if err != nil {
		t.Fatal(err)
	}
	parses, analyses := session.Counts()
	if parses != beforeParses || analyses != beforeAnalyses {
		t.Fatalf("diagnostics rebuilt frontend: %d/%d -> %d/%d", beforeParses, beforeAnalyses, parses, analyses)
	}
	direct, err := RunWorkspaceCheck(string(pathToURI(path)), source, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(diagnostics, direct) {
		t.Fatalf("prepared and direct diagnostics differ: %#v / %#v", diagnostics, direct)
	}
	if len(diagnostics[pathToURI(path)]) != 1 {
		t.Fatalf("expected deprecation warning: %#v", diagnostics)
	}
}

func TestDiagnosticFrontendRunsWhileBackendCheckIsLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	source := "package main\nclass Ready {}\n"
	session := frontend.NewSession()
	compilerCheckMu.Lock()
	locked := true
	defer func() {
		if locked {
			compilerCheckMu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		_, err := runWorkspaceCheckWithFrontend(string(pathToURI(path)), source, nil, "", nil, session)
		done <- err
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
	for {
		_, analyses := session.Counts()
		if analyses > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("diagnostic check ended before frontend analysis: %v", err)
		case <-ticker.C:
		case <-deadline:
			t.Fatal("frontend analysis waited for backend lock")
		}
	}
	compilerCheckMu.Unlock()
	locked = false
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("diagnostic check did not finish after backend lock was released")
	}
}

func TestHoverUsesScopedCompilerBindings(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{"package main\nclass Box { func read(self): int32 { let value = se|lf return 1 } }\n", "Box"},
		{"package main\nfunc first(value: string): string { return value }\nfunc second(value: int32): int32 { return val|ue }\n", "int32"},
	} {
		source, line, column := hoverPosition(t, test.source)
		ctx, err := analysis.NewAnalysisContext(filepath.Join(t.TempDir(), "main.gecko"), source)
		if err != nil {
			t.Fatal(err)
		}
		hover := GetHoverInfo(ctx, source, line, column)
		if hover == nil || !strings.Contains(hover.Type, test.want) || strings.Contains(hover.Type, "unknown") {
			t.Fatalf("hover = %#v, want %s", hover, test.want)
		}
	}
}

func TestWorkspaceReferencesReuseFrontendAnalysis(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib.gecko")
	other := filepath.Join(root, "other.gecko")
	if err := os.WriteFile(lib, []byte("package lib\npublic func add(value: int32): int32 { return value }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("package other\nimport lib\nfunc use(): int32 { return lib.add(2) }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib\nfunc use(): int32 { return lib.add(1) }\n"
	server := NewServer()
	uri := pathToURI(path)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "add"))}
	first, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(first) != 3 {
		t.Fatalf("initial references = %#v, %v", first, err)
	}
	parses, analyses := server.documents.frontendSession.Counts()
	second, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(second) != len(first) {
		t.Fatalf("repeated references = %#v, %v", second, err)
	}
	newParses, newAnalyses := server.documents.frontendSession.Counts()
	if parses != newParses || analyses != newAnalyses {
		t.Fatalf("repeated reference query rebuilt frontend: %d/%d to %d/%d", parses, analyses, newParses, newAnalyses)
	}
}
