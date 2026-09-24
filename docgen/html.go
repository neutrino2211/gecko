// spec: spec/modules.md, spec/stdlib.md

package docgen

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
)

// Template functions
var templateFuncs = template.FuncMap{
	"processDoc": processDocComment,
}

// processDocComment converts backtick references to links and paragraphs
func processDocComment(doc string) template.HTML {
	if doc == "" {
		return ""
	}

	// Convert backticks to code tags
	re := regexp.MustCompile("`([^`]+)`")
	doc = re.ReplaceAllString(doc, "<code>$1</code>")

	// Convert double newlines to paragraphs
	paragraphs := strings.Split(doc, "\n\n")
	var result strings.Builder
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			// Convert single newlines to spaces within paragraphs
			p = strings.ReplaceAll(p, "\n", " ")
			result.WriteString("<p>")
			result.WriteString(p)
			result.WriteString("</p>")
		}
	}

	return template.HTML(result.String())
}

// GenerateIndex generates the main index.html
func GenerateIndex(project *ProjectDoc) (string, error) {
	tmpl, err := template.New("index").Funcs(templateFuncs).Parse(indexTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Name     string
		Style    template.CSS
		Packages []PackageDoc
	}{
		Name:     project.Name,
		Style:    template.CSS(styleCSS),
		Packages: project.Packages,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}

// GeneratePackageIndex generates a package's index.html
func GeneratePackageIndex(pkg *PackageDoc) (string, error) {
	tmpl, err := template.New("package").Funcs(templateFuncs).Parse(packageTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Name       string
		Style      template.CSS
		DocComment string
		Classes    []DocItem
		Traits     []DocItem
		Functions  []DocItem
		Fields     []DocItem
	}{
		Name:       pkg.Name,
		Style:      template.CSS(styleCSS),
		DocComment: pkg.DocComment,
		Classes:    pkg.Classes,
		Traits:     pkg.Traits,
		Functions:  pkg.Functions,
		Fields:     pkg.Fields,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}

// GenerateItemPage generates a page for a class or trait
func GenerateItemPage(pkg string, item *DocItem) (string, error) {
	tmpl, err := template.New("item").Funcs(templateFuncs).Parse(itemTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		Package string
		Style   template.CSS
		Item    *DocItem
	}{
		Package: pkg,
		Style:   template.CSS(styleCSS),
		Item:    item,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	return buf.String(), err
}
