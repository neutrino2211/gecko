package docgen

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
)

// HTML templates embedded as strings
const styleCSS = `
:root {
    --bg-color: #1a1a2e;
    --bg-secondary: #16213e;
    --text-color: #eaeaea;
    --text-muted: #a0a0a0;
    --accent-color: #e94560;
    --link-color: #4db5ff;
    --code-bg: #0f0f1a;
    --border-color: #2a2a4a;
    --success-color: #4ade80;
}

* {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg-color);
    color: var(--text-color);
    line-height: 1.6;
}

.container {
    display: flex;
    min-height: 100vh;
}

/* Sidebar */
.sidebar {
    width: 280px;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border-color);
    padding: 20px;
    position: fixed;
    height: 100vh;
    overflow-y: auto;
}

.sidebar h1 {
    font-size: 1.5rem;
    margin-bottom: 10px;
    color: var(--accent-color);
}

.sidebar h1 a {
    color: inherit;
    text-decoration: none;
}

.sidebar .version {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-bottom: 20px;
}

.sidebar h2 {
    font-size: 0.9rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    margin: 20px 0 10px;
}

.sidebar ul {
    list-style: none;
}

.sidebar li {
    margin: 5px 0;
}

.sidebar a {
    color: var(--link-color);
    text-decoration: none;
    font-size: 0.95rem;
}

.sidebar a:hover {
    text-decoration: underline;
}

.sidebar .kind-badge {
    font-size: 0.7rem;
    padding: 2px 6px;
    border-radius: 3px;
    margin-left: 5px;
    background: var(--border-color);
    color: var(--text-muted);
}

/* Main content */
.main {
    margin-left: 280px;
    flex: 1;
    padding: 40px;
    max-width: 900px;
}

.main h1 {
    font-size: 2rem;
    margin-bottom: 10px;
    color: var(--text-color);
}

.main h2 {
    font-size: 1.5rem;
    margin: 30px 0 15px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--border-color);
}

.main h3 {
    font-size: 1.2rem;
    margin: 20px 0 10px;
}

/* Signature blocks */
.signature {
    font-family: "JetBrains Mono", "Fira Code", monospace;
    background: var(--code-bg);
    padding: 15px 20px;
    border-radius: 8px;
    border-left: 3px solid var(--accent-color);
    overflow-x: auto;
    margin: 15px 0;
    font-size: 0.9rem;
}

/* Documentation text */
.doc-comment {
    margin: 15px 0;
    line-height: 1.8;
}

.doc-comment p {
    margin: 10px 0;
}

.doc-comment code {
    font-family: "JetBrains Mono", "Fira Code", monospace;
    background: var(--code-bg);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.9em;
}

/* Item cards */
.item-card {
    background: var(--bg-secondary);
    border-radius: 8px;
    padding: 20px;
    margin: 15px 0;
    border: 1px solid var(--border-color);
}

.item-card h3 {
    margin-top: 0;
}

.item-card .signature {
    margin-top: 10px;
}

/* Badges */
.badge {
    display: inline-block;
    padding: 3px 8px;
    border-radius: 4px;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-right: 8px;
}

.badge-class { background: #3b82f6; }
.badge-trait { background: #8b5cf6; }
.badge-func { background: #10b981; }
.badge-field { background: #f59e0b; }
.badge-public { background: var(--success-color); color: #000; }
.badge-private { background: #ef4444; }
.badge-external { background: #6366f1; }

/* Type params */
.type-params {
    margin: 10px 0;
    padding: 10px;
    background: var(--code-bg);
    border-radius: 4px;
}

.type-params h4 {
    font-size: 0.9rem;
    color: var(--text-muted);
    margin-bottom: 8px;
}

.type-param {
    margin: 5px 0;
}

.type-param .name {
    color: var(--accent-color);
    font-family: monospace;
}

.type-param .constraint {
    color: var(--link-color);
}

/* Arguments table */
.args-table {
    width: 100%;
    border-collapse: collapse;
    margin: 10px 0;
}

.args-table th, .args-table td {
    padding: 8px 12px;
    text-align: left;
    border-bottom: 1px solid var(--border-color);
}

.args-table th {
    color: var(--text-muted);
    font-weight: 500;
    font-size: 0.85rem;
}

.args-table .arg-name {
    font-family: monospace;
    color: var(--accent-color);
}

.args-table .arg-type {
    font-family: monospace;
    color: var(--link-color);
}

/* Source link */
.source-link {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 10px;
}

.source-link a {
    color: var(--link-color);
}

/* Breadcrumb */
.breadcrumb {
    font-size: 0.9rem;
    color: var(--text-muted);
    margin-bottom: 20px;
}

.breadcrumb a {
    color: var(--link-color);
    text-decoration: none;
}

.breadcrumb a:hover {
    text-decoration: underline;
}

/* Empty state */
.empty-state {
    color: var(--text-muted);
    font-style: italic;
    padding: 20px;
}

/* Responsive */
@media (max-width: 768px) {
    .sidebar {
        display: none;
    }
    .main {
        margin-left: 0;
        padding: 20px;
    }
}
`

const indexTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} - Gecko Documentation</title>
    <style>{{.Style}}</style>
</head>
<body>
<div class="container">
    <nav class="sidebar">
        <h1><a href="index.html">{{.Name}}</a></h1>
        <div class="version">Gecko Documentation</div>

        <h2>Packages</h2>
        <ul>
        {{range .Packages}}
            <li><a href="{{.Name}}/index.html">{{.Name}}</a></li>
        {{end}}
        </ul>
    </nav>

    <main class="main">
        <h1>{{.Name}} Documentation</h1>

        <div class="doc-comment">
            <p>Welcome to the {{.Name}} documentation. Select a package from the sidebar to browse.</p>
        </div>

        <h2>Packages</h2>
        {{range .Packages}}
        <div class="item-card">
            <h3><a href="{{.Name}}/index.html">{{.Name}}</a></h3>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment}}</div>{{end}}
        </div>
        {{end}}
    </main>
</div>
</body>
</html>`

const packageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Name}} - Gecko Documentation</title>
    <style>{{.Style}}</style>
</head>
<body>
<div class="container">
    <nav class="sidebar">
        <h1><a href="../index.html">Gecko Docs</a></h1>
        <div class="version">Package: {{.Name}}</div>

        {{if .Classes}}
        <h2>Classes</h2>
        <ul>
        {{range .Classes}}
            <li><a href="{{.Name}}.html">{{.Name}}</a><span class="kind-badge">class</span></li>
        {{end}}
        </ul>
        {{end}}

        {{if .Traits}}
        <h2>Traits</h2>
        <ul>
        {{range .Traits}}
            <li><a href="{{.Name}}.html">{{.Name}}</a><span class="kind-badge">trait</span></li>
        {{end}}
        </ul>
        {{end}}

        {{if .Functions}}
        <h2>Functions</h2>
        <ul>
        {{range .Functions}}
            <li><a href="#fn-{{.Name}}">{{.Name}}</a></li>
        {{end}}
        </ul>
        {{end}}
    </nav>

    <main class="main">
        <div class="breadcrumb">
            <a href="../index.html">Index</a> / {{.Name}}
        </div>

        <h1>{{.Name}}</h1>
        {{if .DocComment}}<div class="doc-comment">{{.DocComment}}</div>{{end}}

        {{if .Classes}}
        <h2>Classes</h2>
        {{range .Classes}}
        <div class="item-card">
            <span class="badge badge-class">class</span>
            {{if eq .Visibility "public"}}<span class="badge badge-public">public</span>{{end}}
            {{if eq .Visibility "private"}}<span class="badge badge-private">private</span>{{end}}
            <h3><a href="{{.Name}}.html">{{.Name}}</a></h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
        </div>
        {{end}}
        {{end}}

        {{if .Traits}}
        <h2>Traits</h2>
        {{range .Traits}}
        <div class="item-card">
            <span class="badge badge-trait">trait</span>
            <h3><a href="{{.Name}}.html">{{.Name}}</a></h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
        </div>
        {{end}}
        {{end}}

        {{if .Functions}}
        <h2>Functions</h2>
        {{range .Functions}}
        <div class="item-card" id="fn-{{.Name}}">
            <span class="badge badge-func">func</span>
            {{if eq .Visibility "external"}}<span class="badge badge-external">external</span>{{end}}
            <h3>{{.Name}}</h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
            {{if .Arguments}}
            <h4>Arguments</h4>
            <table class="args-table">
                <tr><th>Name</th><th>Type</th></tr>
                {{range .Arguments}}
                <tr><td class="arg-name">{{.Name}}</td><td class="arg-type">{{.Type}}</td></tr>
                {{end}}
            </table>
            {{end}}
            {{if and .ReturnType (ne .ReturnType "void")}}
            <h4>Returns</h4>
            <p><code>{{.ReturnType}}</code></p>
            {{end}}
        </div>
        {{end}}
        {{end}}

        {{if .Fields}}
        <h2>Global Fields</h2>
        {{range .Fields}}
        <div class="item-card">
            <span class="badge badge-field">field</span>
            <h3>{{.Name}}</h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
        </div>
        {{end}}
        {{end}}
    </main>
</div>
</body>
</html>`

const itemTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Item.Name}} - {{.Package}} - Gecko Documentation</title>
    <style>{{.Style}}</style>
</head>
<body>
<div class="container">
    <nav class="sidebar">
        <h1><a href="../index.html">Gecko Docs</a></h1>
        <div class="version">{{.Package}}.{{.Item.Name}}</div>

        {{if .Item.Fields}}
        <h2>Fields</h2>
        <ul>
        {{range .Item.Fields}}
            <li><a href="#field-{{.Name}}">{{.Name}}</a></li>
        {{end}}
        </ul>
        {{end}}

        {{if .Item.Methods}}
        <h2>Methods</h2>
        <ul>
        {{range .Item.Methods}}
            <li><a href="#method-{{.Name}}">{{.Name}}</a></li>
        {{end}}
        </ul>
        {{end}}
    </nav>

    <main class="main">
        <div class="breadcrumb">
            <a href="../index.html">Index</a> / <a href="index.html">{{.Package}}</a> / {{.Item.Name}}
        </div>

        <h1>
            <span class="badge badge-{{.Item.Kind}}">{{.Item.Kind}}</span>
            {{.Item.Name}}
        </h1>

        <pre class="signature">{{.Item.Signature}}</pre>

        {{if .Item.DocComment}}
        <div class="doc-comment">{{.Item.DocComment | processDoc}}</div>
        {{end}}

        {{if .Item.TypeParams}}
        <div class="type-params">
            <h4>Type Parameters</h4>
            {{range .Item.TypeParams}}
            <div class="type-param">
                <span class="name">{{.Name}}</span>
                {{if .Constraint}} : <span class="constraint">{{.Constraint}}</span>{{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        <p class="source-link">
            Defined in <a href="#">{{.Item.SourceFile}}:{{.Item.Line}}</a>
        </p>

        {{if .Item.Fields}}
        <h2>Fields</h2>
        {{range .Item.Fields}}
        <div class="item-card" id="field-{{.Name}}">
            <span class="badge badge-field">field</span>
            <h3>{{.Name}}</h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
        </div>
        {{end}}
        {{end}}

        {{if .Item.Methods}}
        <h2>Methods</h2>
        {{range .Item.Methods}}
        <div class="item-card" id="method-{{.Name}}">
            <span class="badge badge-func">method</span>
            <h3>{{.Name}}</h3>
            <pre class="signature">{{.Signature}}</pre>
            {{if .DocComment}}<div class="doc-comment">{{.DocComment | processDoc}}</div>{{end}}
            {{if .Arguments}}
            <h4>Arguments</h4>
            <table class="args-table">
                <tr><th>Name</th><th>Type</th></tr>
                {{range .Arguments}}
                <tr><td class="arg-name">{{.Name}}</td><td class="arg-type">{{.Type}}</td></tr>
                {{end}}
            </table>
            {{end}}
            {{if and .ReturnType (ne .ReturnType "void")}}
            <h4>Returns</h4>
            <p><code>{{.ReturnType}}</code></p>
            {{end}}
        </div>
        {{end}}
        {{end}}
    </main>
</div>
</body>
</html>`

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
