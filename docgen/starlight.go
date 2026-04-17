package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateStarlight generates markdown files for Astro Starlight
func GenerateStarlight(project *ProjectDoc, outputDir string) error {
	docsDir := filepath.Join(outputDir, "src", "content", "docs")
	stdlibDir := filepath.Join(docsDir, "stdlib")

	if err := os.MkdirAll(stdlibDir, 0755); err != nil {
		return fmt.Errorf("failed to create docs directory: %w", err)
	}

	// Write index page
	if err := writeStarlightIndex(docsDir, project); err != nil {
		return err
	}

	// Write stdlib index
	if err := writeStarlightStdlibIndex(stdlibDir, project); err != nil {
		return err
	}

	// Write each package/class
	for _, pkg := range project.Packages {
		for _, class := range pkg.Classes {
			if err := writeStarlightClass(stdlibDir, pkg.Name, &class); err != nil {
				return err
			}
		}
		for _, trait := range pkg.Traits {
			if err := writeStarlightTrait(stdlibDir, pkg.Name, &trait); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeStarlightIndex(dir string, _ *ProjectDoc) error {
	var sb strings.Builder

	sb.WriteString(`---
title: Gecko Documentation
description: Documentation for the Gecko programming language
template: splash
hero:
  tagline: A systems programming language with TypeScript-like ergonomics
  actions:
    - text: Get Started
      link: /stdlib/
      icon: right-arrow
---

## Standard Library

The Gecko standard library provides essential types for memory management:

| Type | Description |
|------|-------------|
| **Box\<T\>** | Unique ownership heap allocation |
| **Rc\<T\>** | Reference-counted shared ownership |
| **Weak\<T\>** | Non-owning references to Rc data |
| **Raw\<T\>** | Unsafe pointer wrapper for low-level operations |

## Quick Example

` + "```ts\n" + `import box

let b: Box<int32> = Box<int32>::new(42)
let val: int32 = b.get()
b.drop()
` + "```\n")

	return os.WriteFile(filepath.Join(dir, "index.mdx"), []byte(sb.String()), 0644)
}

func writeStarlightStdlibIndex(dir string, project *ProjectDoc) error {
	var sb strings.Builder

	sb.WriteString(`---
title: Standard Library
description: Gecko standard library reference
---

The standard library provides memory management primitives for Gecko programs.

## Packages

`)

	for _, pkg := range project.Packages {
		sb.WriteString(fmt.Sprintf("### %s\n\n", pkg.Name))

		for _, class := range pkg.Classes {
			firstLine := getFirstLine(class.DocComment)
			sb.WriteString(fmt.Sprintf("- [**%s**](/stdlib/%s-%s/) - %s\n", class.Name, pkg.Name, strings.ToLower(class.Name), firstLine))
		}
		sb.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(dir, "index.md"), []byte(sb.String()), 0644)
}

func writeStarlightClass(dir, pkgName string, item *DocItem) error {
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.md", pkgName, strings.ToLower(item.Name)))

	var sb strings.Builder

	// Frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %s\n", item.Name))
	firstLine := getFirstLine(item.DocComment)
	if firstLine != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", firstLine))
	}
	sb.WriteString("---\n\n")

	// Signature
	sb.WriteString("```ts\n")
	sb.WriteString(item.Signature)
	sb.WriteString("\n```\n\n")

	// Doc comment
	if item.DocComment != "" {
		sb.WriteString(item.DocComment)
		sb.WriteString("\n\n")
	}

	// Type params
	if len(item.TypeParams) > 0 {
		sb.WriteString("## Type Parameters\n\n")
		for _, tp := range item.TypeParams {
			if tp.Constraint != "" {
				sb.WriteString(fmt.Sprintf("- **%s** - constrained by `%s`\n", tp.Name, tp.Constraint))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**\n", tp.Name))
			}
		}
		sb.WriteString("\n")
	}

	// Fields
	if len(item.Fields) > 0 {
		sb.WriteString("## Fields\n\n")
		for _, f := range item.Fields {
			sb.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
			sb.WriteString("```ts\n")
			sb.WriteString(f.Signature)
			sb.WriteString("\n```\n\n")
			if f.DocComment != "" {
				sb.WriteString(f.DocComment)
				sb.WriteString("\n\n")
			}
		}
	}

	// Methods
	if len(item.Methods) > 0 {
		sb.WriteString("## Methods\n\n")
		for _, m := range item.Methods {
			sb.WriteString(fmt.Sprintf("### %s\n\n", m.Name))
			sb.WriteString("```ts\n")
			sb.WriteString(m.Signature)
			sb.WriteString("\n```\n\n")
			if m.DocComment != "" {
				sb.WriteString(m.DocComment)
				sb.WriteString("\n\n")
			}

			// Arguments
			if len(m.Arguments) > 0 {
				sb.WriteString("**Arguments:**\n\n")
				sb.WriteString("| Name | Type |\n")
				sb.WriteString("|------|------|\n")
				for _, arg := range m.Arguments {
					sb.WriteString(fmt.Sprintf("| `%s` | `%s` |\n", arg.Name, arg.Type))
				}
				sb.WriteString("\n")
			}

			// Return type
			if m.ReturnType != "" && m.ReturnType != "void" {
				sb.WriteString(fmt.Sprintf("**Returns:** `%s`\n\n", m.ReturnType))
			}
		}
	}

	// Source
	if item.SourceFile != "" {
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("*Defined in `%s:%d`*\n", item.SourceFile, item.Line))
	}

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

func writeStarlightTrait(dir, pkgName string, item *DocItem) error {
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s.md", pkgName, strings.ToLower(item.Name)))

	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %s (trait)\n", item.Name))
	firstLine := getFirstLine(item.DocComment)
	if firstLine != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", firstLine))
	}
	sb.WriteString("---\n\n")

	sb.WriteString("```ts\n")
	sb.WriteString(item.Signature)
	sb.WriteString("\n```\n\n")

	if item.DocComment != "" {
		sb.WriteString(item.DocComment)
		sb.WriteString("\n\n")
	}

	if len(item.Methods) > 0 {
		sb.WriteString("## Required Methods\n\n")
		for _, m := range item.Methods {
			sb.WriteString(fmt.Sprintf("### %s\n\n", m.Name))
			sb.WriteString("```ts\n")
			sb.WriteString(m.Signature)
			sb.WriteString("\n```\n\n")
			if m.DocComment != "" {
				sb.WriteString(m.DocComment)
				sb.WriteString("\n\n")
			}
		}
	}

	return os.WriteFile(filename, []byte(sb.String()), 0644)
}

func getFirstLine(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
