package commands

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}

	return string(out)
}

func TestGenerateMainGeckoUsesExternalMain(t *testing.T) {
	content := generateMainGecko("myapp")

	if !strings.Contains(content, "external func main(): int32") {
		t.Fatalf("expected generated main.gecko to contain external main function, got:\n%s", content)
	}
}

func TestInitExePrintsValidRunCommandAndGeneratesExternalMain(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	fs := flag.NewFlagSet("init-test", flag.ContinueOnError)
	fs.String("type", "exe", "")
	if err := fs.Set("type", "exe"); err != nil {
		t.Fatalf("failed to set project type flag: %v", err)
	}
	if err := fs.Parse([]string{"demo"}); err != nil {
		t.Fatalf("failed to parse args: %v", err)
	}

	ctx := cli.NewContext(cli.NewApp(), fs, nil)
	output := captureStdout(t, func() {
		if err := initProject(ctx); err != nil {
			t.Fatalf("initProject failed: %v", err)
		}
	})

	if !strings.Contains(output, "gecko run src/main.gecko") {
		t.Fatalf("expected success output to contain valid run command, got:\n%s", output)
	}
	if strings.Contains(output, "gecko run --entry main") {
		t.Fatalf("output contains obsolete run command:\n%s", output)
	}

	mainSourcePath := filepath.Join("demo", "src", "main.gecko")
	mainSourceBytes, err := os.ReadFile(mainSourcePath)
	if err != nil {
		t.Fatalf("failed to read generated main.gecko: %v", err)
	}

	mainSource := string(mainSourceBytes)
	if !strings.Contains(mainSource, "external func main(): int32") {
		t.Fatalf("expected generated main.gecko to define external main function, got:\n%s", mainSource)
	}
}
