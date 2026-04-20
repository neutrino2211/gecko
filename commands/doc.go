package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/docgen"
	"github.com/neutrino2211/gecko/parser"
	"github.com/urfave/cli/v2"
)

var DocCommand = &cli.Command{
	Name:        "doc",
	Aliases:     []string{"d"},
	Usage:       "Generate documentation from Gecko source files",
	Description: "Parses Gecko source files and generates documentation (Astro/Markdown or HTML)",
	Action:      docAction,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "./docs",
			Usage:   "Output directory for generated documentation",
		},
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Value:   "Project",
			Usage:   "Project name for documentation",
		},
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "astro",
			Usage:   "Output format: 'astro' (markdown for Astro) or 'html'",
		},
		&cli.BoolFlag{
			Name:  "private",
			Value: false,
			Usage: "Include private items in documentation",
		},
	},
}

func docAction(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("usage: gecko doc <file_or_directory>")
	}

	input := ctx.Args().Get(0)
	outputDir := ctx.String("output")
	projectName := ctx.String("name")
	format := ctx.String("format")

	// Find all .gecko files
	var files []string
	info, err := os.Stat(input)
	if err != nil {
		return fmt.Errorf("cannot access %s: %v", input, err)
	}

	if info.IsDir() {
		err = filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".gecko") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error walking directory: %v", err)
		}
	} else {
		files = append(files, input)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .gecko files found in %s", input)
	}

	fmt.Printf("Generating documentation for %d file(s)...\n", len(files))

	// Parse all files and extract documentation
	project := &docgen.ProjectDoc{
		Name:     projectName,
		Packages: []docgen.PackageDoc{},
	}

	packageMap := make(map[string]*docgen.PackageDoc)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Warning: cannot read %s: %v\n", file, err)
			continue
		}

		parsed, err := parser.Parser.ParseString(file, string(content))
		if err != nil {
			fmt.Printf("Warning: cannot parse %s: %v\n", file, err)
			continue
		}

		pkgDoc := docgen.ExtractPackageDoc(parsed, file)

		// Merge with existing package if already seen
		if existing, ok := packageMap[pkgDoc.Name]; ok {
			existing.Classes = append(existing.Classes, pkgDoc.Classes...)
			existing.Traits = append(existing.Traits, pkgDoc.Traits...)
			existing.Functions = append(existing.Functions, pkgDoc.Functions...)
			existing.Fields = append(existing.Fields, pkgDoc.Fields...)
		} else {
			packageMap[pkgDoc.Name] = &pkgDoc
		}
	}

	// Convert map to slice
	for _, pkg := range packageMap {
		project.Packages = append(project.Packages, *pkg)
	}

	// Generate based on format
	if format == "astro" || format == "starlight" {
		err = docgen.GenerateStarlight(project, outputDir)
		if err != nil {
			return fmt.Errorf("error generating starlight docs: %v", err)
		}
		fmt.Printf("Starlight docs generated in %s/src/content/docs\n", outputDir)
		fmt.Printf("Run 'npm run dev' in %s to preview\n", outputDir)
		return nil
	}

	// HTML format (legacy)
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("cannot create output directory: %v", err)
	}

	indexHTML, err := docgen.GenerateIndex(project)
	if err != nil {
		return fmt.Errorf("error generating index: %v", err)
	}
	err = os.WriteFile(filepath.Join(outputDir, "index.html"), []byte(indexHTML), 0644)
	if err != nil {
		return fmt.Errorf("error writing index.html: %v", err)
	}

	for _, pkg := range project.Packages {
		pkgDir := filepath.Join(outputDir, pkg.Name)
		err = os.MkdirAll(pkgDir, 0755)
		if err != nil {
			return fmt.Errorf("cannot create package directory: %v", err)
		}

		pkgHTML, err := docgen.GeneratePackageIndex(&pkg)
		if err != nil {
			return fmt.Errorf("error generating package index: %v", err)
		}
		err = os.WriteFile(filepath.Join(pkgDir, "index.html"), []byte(pkgHTML), 0644)
		if err != nil {
			return fmt.Errorf("error writing package index: %v", err)
		}

		for _, class := range pkg.Classes {
			classHTML, err := docgen.GenerateItemPage(pkg.Name, &class)
			if err != nil {
				return fmt.Errorf("error generating class page: %v", err)
			}
			err = os.WriteFile(filepath.Join(pkgDir, class.Name+".html"), []byte(classHTML), 0644)
			if err != nil {
				return fmt.Errorf("error writing class page: %v", err)
			}
		}

		for _, trait := range pkg.Traits {
			traitHTML, err := docgen.GenerateItemPage(pkg.Name, &trait)
			if err != nil {
				return fmt.Errorf("error generating trait page: %v", err)
			}
			err = os.WriteFile(filepath.Join(pkgDir, trait.Name+".html"), []byte(traitHTML), 0644)
			if err != nil {
				return fmt.Errorf("error writing trait page: %v", err)
			}
		}
	}

	fmt.Printf("Documentation generated in %s\n", outputDir)
	fmt.Printf("Open %s/index.html to view\n", outputDir)

	return nil
}
