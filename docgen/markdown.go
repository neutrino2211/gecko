package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateMarkdown generates markdown files for the Astro docs site
func GenerateMarkdown(project *ProjectDoc, outputDir string) error {
	contentDir := filepath.Join(outputDir, "src", "content", "packages")

	if err := os.MkdirAll(contentDir, 0755); err != nil {
		return fmt.Errorf("failed to create content directory: %w", err)
	}

	for _, pkg := range project.Packages {
		for _, class := range pkg.Classes {
			if err := writeClassMarkdown(contentDir, pkg.Name, &class); err != nil {
				return err
			}
		}
		for _, trait := range pkg.Traits {
			if err := writeTraitMarkdown(contentDir, pkg.Name, &trait); err != nil {
				return err
			}
		}
		for _, fn := range pkg.Functions {
			if err := writeFunctionMarkdown(contentDir, pkg.Name, &fn); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeClassMarkdown(dir, pkgName string, item *DocItem) error {
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.md", pkgName, item.Name))

	var sb strings.Builder

	// Frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", item.Name))
	sb.WriteString(fmt.Sprintf("package: %q\n", pkgName))
	sb.WriteString("kind: \"class\"\n")
	sb.WriteString(fmt.Sprintf("signature: %q\n", item.Signature))

	if item.SourceFile != "" {
		sb.WriteString(fmt.Sprintf("sourceFile: %q\n", item.SourceFile))
		sb.WriteString(fmt.Sprintf("line: %d\n", item.Line))
	}

	// Type params
	if len(item.TypeParams) > 0 {
		sb.WriteString("typeParams:\n")
		for _, tp := range item.TypeParams {
			sb.WriteString(fmt.Sprintf("  - name: %q\n", tp.Name))
			if tp.Constraint != "" {
				sb.WriteString(fmt.Sprintf("    constraint: %q\n", tp.Constraint))
			}
		}
	}

	// Fields
	if len(item.Fields) > 0 {
		sb.WriteString("fields:\n")
		for _, f := range item.Fields {
			sb.WriteString(fmt.Sprintf("  - name: %q\n", f.Name))
			sb.WriteString(fmt.Sprintf("    signature: %q\n", f.Signature))
			if f.DocComment != "" {
				sb.WriteString(fmt.Sprintf("    doc: %q\n", escapeYamlString(f.DocComment)))
			}
			if f.FieldType != "" {
				sb.WriteString(fmt.Sprintf("    type: %q\n", f.FieldType))
			}
		}
	}

	// Methods
	if len(item.Methods) > 0 {
		sb.WriteString("methods:\n")
		for _, m := range item.Methods {
			sb.WriteString(fmt.Sprintf("  - name: %q\n", m.Name))
			sb.WriteString(fmt.Sprintf("    signature: %q\n", m.Signature))
			if m.DocComment != "" {
				sb.WriteString(fmt.Sprintf("    doc: %q\n", escapeYamlString(m.DocComment)))
			}
			if len(m.Arguments) > 0 {
				sb.WriteString("    args:\n")
				for _, arg := range m.Arguments {
					sb.WriteString(fmt.Sprintf("      - name: %q\n", arg.Name))
					sb.WriteString(fmt.Sprintf("        type: %q\n", arg.Type))
				}
			}
			if m.ReturnType != "" {
				sb.WriteString(fmt.Sprintf("    returnType: %q\n", m.ReturnType))
			}
		}
	}

	sb.WriteString("---\n\n")

	// Body content (doc comment)
	if item.DocComment != "" {
		sb.WriteString(formatDocComment(item.DocComment))
	}

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

func writeTraitMarkdown(dir, pkgName string, item *DocItem) error {
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.md", pkgName, item.Name))

	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", item.Name))
	sb.WriteString(fmt.Sprintf("package: %q\n", pkgName))
	sb.WriteString("kind: \"trait\"\n")
	sb.WriteString(fmt.Sprintf("signature: %q\n", item.Signature))

	if item.SourceFile != "" {
		sb.WriteString(fmt.Sprintf("sourceFile: %q\n", item.SourceFile))
		sb.WriteString(fmt.Sprintf("line: %d\n", item.Line))
	}

	if len(item.Methods) > 0 {
		sb.WriteString("methods:\n")
		for _, m := range item.Methods {
			sb.WriteString(fmt.Sprintf("  - name: %q\n", m.Name))
			sb.WriteString(fmt.Sprintf("    signature: %q\n", m.Signature))
			if m.DocComment != "" {
				sb.WriteString(fmt.Sprintf("    doc: %q\n", escapeYamlString(m.DocComment)))
			}
			if len(m.Arguments) > 0 {
				sb.WriteString("    args:\n")
				for _, arg := range m.Arguments {
					sb.WriteString(fmt.Sprintf("      - name: %q\n", arg.Name))
					sb.WriteString(fmt.Sprintf("        type: %q\n", arg.Type))
				}
			}
			if m.ReturnType != "" {
				sb.WriteString(fmt.Sprintf("    returnType: %q\n", m.ReturnType))
			}
		}
	}

	sb.WriteString("---\n\n")

	if item.DocComment != "" {
		sb.WriteString(formatDocComment(item.DocComment))
	}

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

func writeFunctionMarkdown(dir, pkgName string, item *DocItem) error {
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.md", pkgName, item.Name))

	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", item.Name))
	sb.WriteString(fmt.Sprintf("package: %q\n", pkgName))
	sb.WriteString("kind: \"function\"\n")
	sb.WriteString(fmt.Sprintf("signature: %q\n", item.Signature))

	if item.SourceFile != "" {
		sb.WriteString(fmt.Sprintf("sourceFile: %q\n", item.SourceFile))
		sb.WriteString(fmt.Sprintf("line: %d\n", item.Line))
	}

	if len(item.TypeParams) > 0 {
		sb.WriteString("typeParams:\n")
		for _, tp := range item.TypeParams {
			sb.WriteString(fmt.Sprintf("  - name: %q\n", tp.Name))
			if tp.Constraint != "" {
				sb.WriteString(fmt.Sprintf("    constraint: %q\n", tp.Constraint))
			}
		}
	}

	sb.WriteString("---\n\n")

	if item.DocComment != "" {
		sb.WriteString(formatDocComment(item.DocComment))
	}

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

func escapeYamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	// Collapse multiple newlines into single space for inline YAML
	s = strings.ReplaceAll(s, "\n\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func formatDocComment(doc string) string {
	lines := strings.Split(doc, "\n")
	var result strings.Builder

	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		if inCodeBlock {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Empty lines become paragraph breaks
		if trimmed == "" {
			result.WriteString("\n")
			continue
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
