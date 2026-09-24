// spec: spec/modules.md, spec/stdlib.md

package docgen

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
