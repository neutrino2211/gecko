package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

var InitCommand = &cli.Command{
	Name:      "init",
	Aliases:   []string{"i"},
	Usage:     "Initialize a new Gecko project",
	ArgsUsage: "[project-name]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "type",
			Aliases: []string{"t"},
			Value:   "exe",
			Usage:   "Project type: exe (executable) or lib (library)",
		},
	},
	Action: initProject,
}

func initProject(c *cli.Context) error {
	projectName := c.Args().First()
	projectType := c.String("type")

	// Validate project type
	if projectType != "exe" && projectType != "lib" {
		return fmt.Errorf("invalid project type '%s': must be 'exe' or 'lib'", projectType)
	}

	// Determine project directory
	var projectDir string
	if projectName == "" {
		// Use current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		projectDir = cwd
		projectName = filepath.Base(cwd)
	} else {
		// Create new directory
		projectDir = projectName
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			return fmt.Errorf("failed to create project directory: %w", err)
		}
	}

	// Check if gecko.toml already exists
	tomlPath := filepath.Join(projectDir, "gecko.toml")
	if _, err := os.Stat(tomlPath); err == nil {
		return fmt.Errorf("gecko.toml already exists in %s", projectDir)
	}

	// Create src directory
	srcDir := filepath.Join(projectDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	// Generate gecko.toml
	tomlContent := generateGeckoToml(projectName, projectType)
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
		return fmt.Errorf("failed to write gecko.toml: %w", err)
	}

	// Generate main source file
	var srcFile, srcContent string
	if projectType == "exe" {
		srcFile = filepath.Join(srcDir, "main.gecko")
		srcContent = generateMainGecko(projectName)
	} else {
		srcFile = filepath.Join(srcDir, "lib.gecko")
		srcContent = generateLibGecko(projectName)
	}

	if err := os.WriteFile(srcFile, []byte(srcContent), 0644); err != nil {
		return fmt.Errorf("failed to write source file: %w", err)
	}

	// Create .gitignore
	gitignorePath := filepath.Join(projectDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignoreContent := generateGitignore()
		if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	// Print success message
	fmt.Printf("Created %s project '%s' in %s\n", projectType, projectName, projectDir)
	fmt.Println()
	fmt.Println("To get started:")
	if projectName != filepath.Base(projectDir) || c.Args().First() != "" {
		fmt.Printf("  cd %s\n", projectName)
	}
	if projectType == "exe" {
		fmt.Println("  gecko run --entry main")
	} else {
		fmt.Println("  gecko build --entry lib")
	}

	return nil
}

func generateGeckoToml(name, projectType string) string {
	var sb strings.Builder

	sb.WriteString("[package]\n")
	sb.WriteString(fmt.Sprintf("name = \"%s\"\n", name))
	sb.WriteString("version = \"0.1.0\"\n")
	sb.WriteString("\n")

	sb.WriteString("[build]\n")
	sb.WriteString("backend = \"c\"\n")
	sb.WriteString("\n")

	sb.WriteString("[build.entries]\n")
	if projectType == "exe" {
		sb.WriteString("main = \"src/main.gecko\"\n")
	} else {
		sb.WriteString("lib = \"src/lib.gecko\"\n")
	}
	sb.WriteString("\n")

	sb.WriteString("[build.profiles.debug]\n")
	sb.WriteString("optimize = false\n")
	sb.WriteString("\n")

	sb.WriteString("[build.profiles.release]\n")
	sb.WriteString("optimize = true\n")

	return sb.String()
}

func generateMainGecko(name string) string {
	return fmt.Sprintf(`// %s - A Gecko executable

declare external func puts(s: string): int32

func main(): int32 {
    puts("Hello from %s!")
    return 0
}
`, name, name)
}

func generateLibGecko(name string) string {
	return fmt.Sprintf(`// %s - A Gecko library

// Export public functions and types for consumers of this library

public func add(a: int32, b: int32): int32 {
    return a + b
}

public class Point {
    x: int32
    y: int32
}

impl Point {
    public func new(x: int32, y: int32): Point {
        return Point { x: x, y: y }
    }

    public func distance(self, other: Point*): int32 {
        let dx = self.x - other.x
        let dy = self.y - other.y
        return dx * dx + dy * dy
    }
}
`, name)
}

func generateGitignore() string {
	return `# Build artifacts
*.o
*.c
*.ll
*.out

# Gecko build directories
.gecko/
build/

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
`
}
