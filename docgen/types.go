package docgen

// DocItem represents a documented item (class, method, field, trait, etc.)
type DocItem struct {
	Name        string
	Kind        string // "class", "method", "field", "trait", "enum"
	DocComment  string // Joined doc comment lines
	Signature   string // Full type signature
	Visibility  string // "public", "private", "protected", "external"
	SourceFile  string
	Line        int

	// For classes and traits
	TypeParams []TypeParamDoc
	Fields     []DocItem
	Methods    []DocItem

	// For methods
	Arguments  []ArgDoc
	ReturnType string

	// For fields
	FieldType string
	IsMutable bool // let vs const
}

// TypeParamDoc represents a generic type parameter
type TypeParamDoc struct {
	Name       string
	Constraint string // trait constraint, if any
	DocComment string
}

// ArgDoc represents a function/method argument
type ArgDoc struct {
	Name string
	Type string
}

// PackageDoc represents a documented package
type PackageDoc struct {
	Name       string
	SourceFile string
	DocComment string // Package-level doc comment
	Classes    []DocItem
	Traits     []DocItem
	Functions  []DocItem
	Fields     []DocItem // Global fields
}

// ProjectDoc represents the entire project documentation
type ProjectDoc struct {
	Name     string
	Packages []PackageDoc
}
