// spec: spec/modules.md, spec/stdlib.md

package docgen

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
