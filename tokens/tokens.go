// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/config"
)

type baseToken struct {
	Pos    lexer.Position
	EndPos lexer.Position
	RefID  string
}

// UnsafeBlockErrorNames maps an `@unsafe with ...` block to the synthesized C
// error-enum type name (variants = handler ids) produced by the analyzer. It is a
// package-level map (rather than a `parser:"-"` field on UnsafeBlock) because
// participle's LL(1) builder rejects an ignored field on a
// struct that also carries an `@@*` body.
var UnsafeBlockErrorNames = map[*UnsafeBlock]string{}

// Attribute represents a compile-time attribute like @packed, @section(".text"), or @drop_hook(.drop)
type Attribute struct {
	baseToken
	Name string          `parser:"'@' @Ident"`
	Args []*AttributeArg `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
}

// AttributeArg represents an argument to an attribute - either a string or a method reference
type AttributeArg struct {
	baseToken
	String string `parser:"@String"`
	Method string `parser:"| '.' @Ident"`
}

// GetStringValue returns the first string argument value (for backwards compat with @section(".text"))
func (a *Attribute) GetStringValue() string {
	if len(a.Args) > 0 && a.Args[0].String != "" {
		v := a.Args[0].String
		// Remove quotes from the value
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
		}
		return v
	}
	return ""
}

// GetHookMethods returns all method references (for hook attributes like @drop_hook(.drop))
func (a *Attribute) GetHookMethods() []string {
	var methods []string
	for _, arg := range a.Args {
		if arg.Method != "" {
			methods = append(methods, arg.Method)
		}
	}
	return methods
}

// File tokens

// DirectoryImport represents a lazily-resolved directory import
type DirectoryImport struct {
	Path       string   // Full import path (e.g., "std.collections")
	DirPath    string   // Filesystem path to the directory
	UseObjects []string // Specific symbols to import (empty = all public)
	Alias      string   // Import alias (e.g., "Vec" from `import std.collections as Vec`)
}

type File struct {
	Attributes       []*Attribute `parser:"{ @@ }"`
	PackageName      string       `parser:"['package' @Ident]"`
	Entries          []*Entry     `parser:"@@*"`
	Imports          []*File
	DirectoryImports []*DirectoryImport // Lazily resolved directory imports
	Config           *config.CompileCfg
	Name             string
	Path             string
	Content          string
	Alias            string // Import alias (e.g., "Vec" from `import std.collections as Vec`)
}

// GetBackend returns the backend specified by @backend attribute, or empty string if not specified
func (f *File) GetBackend() string {
	for _, attr := range f.Attributes {
		if attr.Name == "backend" {
			return attr.GetStringValue()
		}
	}
	return ""
}

type CImport struct {
	baseToken
	Header  string           `parser:"'cimport' @String"`
	Clauses []*CImportClause `parser:"{ @@ }"`
}

type CImportClause struct {
	baseToken
	WithObject  string `parser:"'withobject' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String"`
}

type Foreign struct {
	baseToken
	Backend string           `parser:"'foreign' @String"`
	Module  string           `parser:"@Ident"`
	Clauses []*ForeignClause `parser:"{ @@ }"`
	Members []*ForeignMember `parser:"'{' { @@ } '}'"`
}

type ForeignClause struct {
	baseToken
	WithHeader  string `parser:"'withheader' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String"`
	WithObject  string `parser:"| 'withobject' @String"`
}

type ForeignMember struct {
	baseToken
	Type   *ForeignType   `parser:"@@"`
	Method *ForeignMethod `parser:"| @@"`
}

type ForeignType struct {
	baseToken
	Name string `parser:"'type' @Ident 'opaque'"`
}

type ForeignMethod struct {
	baseToken
	Name      string   `parser:"'func' @Ident"`
	Arguments []*Value `parser:"'(' [ @@ { ',' @@ } ]"`
	Variadic  bool     `parser:"[ ',' @'...' | @'...' ] ')'"`
	Type      *TypeRef `parser:"[ ':' @@ ]"`
	Throws    *TypeRef `parser:"[ 'throws' @@ ]"`
	As        string   `parser:"[ 'as' @String ]"`
}

func (m *ForeignMethod) IsVariadic() bool {
	if m == nil {
		return false
	}
	return m.Variadic
}

// GetWithObjects returns all withobject values in declaration order.
func (c *CImport) GetWithObjects() []string {
	var objects []string
	for _, clause := range c.Clauses {
		if clause.WithObject != "" {
			objects = append(objects, clause.WithObject)
		}
	}
	return objects
}

// GetWithLibraries returns all withlibrary values in declaration order.
func (c *CImport) GetWithLibraries() []string {
	var libs []string
	for _, clause := range c.Clauses {
		if clause.WithLibrary != "" {
			libs = append(libs, clause.WithLibrary)
		}
	}
	return libs
}

func (f *Foreign) GetWithHeaders() []string {
	var headers []string
	for _, clause := range f.Clauses {
		if clause.WithHeader != "" {
			headers = append(headers, clause.WithHeader)
		}
	}
	return headers
}

func (f *Foreign) GetWithLibraries() []string {
	var libs []string
	for _, clause := range f.Clauses {
		if clause.WithLibrary != "" {
			libs = append(libs, clause.WithLibrary)
		}
	}
	return libs
}

func (f *Foreign) GetWithObjects() []string {
	var objects []string
	for _, clause := range f.Clauses {
		if clause.WithObject != "" {
			objects = append(objects, clause.WithObject)
		}
	}
	return objects
}

type Import struct {
	baseToken
	Path    []string `parser:"'import' @Ident { '.' @Ident }"`
	Alias   string   `parser:"[ 'as' @Ident ]"`
	Objects []string `parser:"['use' '{' [ @Ident { ',' @Ident } ] '}']"`
}

// Package returns the full dot-separated import path as a string
func (i *Import) Package() string {
	if len(i.Path) == 0 {
		return ""
	}
	result := i.Path[0]
	for _, p := range i.Path[1:] {
		result += "." + p
	}
	return result
}

// ModuleName returns the alias if set, otherwise the last component of the import path
func (i *Import) ModuleName() string {
	if i.Alias != "" {
		return i.Alias
	}
	if len(i.Path) == 0 {
		return ""
	}
	return i.Path[len(i.Path)-1]
}

type Entry struct {
	baseToken
	Return     *Expression `parser:"'return' @@"`
	VoidReturn *bool       `parser:"| @'return'"`
	Break      *bool       `parser:"| @'break'"`
	Continue   *bool       `parser:"| @'continue'"`
	// Keyword-led statements that begin with a bare identifier must precede
	// ExprStmt. Expression parsing greedily consumes a leading identifier, so
	// `import ...`, `cimport ...`, `asm ...` and `impl ...` would otherwise be
	// misread as an expression statement and never reach their own alternatives.
	Import  *Import  `parser:"| @@"`
	CImport *CImport `parser:"| @@"`
	Asm     *Asm     `parser:"| @@"`
	// Implementation must come before Assignment to prevent 'impl' being parsed as identifier
	Implementation *Implementation `parser:"| @@"`
	Assignment     *Assignment     `parser:"| @@"`
	ElseIf         *ElseIf         `parser:"| @@"`
	Else           *Else           `parser:"| @@"`
	If             *If             `parser:"| @@"`
	Match          *Match          `parser:"| @@"`
	Defer          *Defer          `parser:"| @@"`
	// Declarations with optional attributes must come before Intrinsic
	// so @attr func/class/trait is parsed as declaration, not intrinsic
	Class  *Class  `parser:"| @@"`
	Trait  *Trait  `parser:"| @@"`
	Method *Method `parser:"| @@"`
	// Destructuring declarations must precede Field: `let Point { x, y } = p`
	// otherwise Field consumes `let Point` and leaves the braces unconsumed.
	Destructuring *DestructuringDeclaration `parser:"| @@"`
	Field         *Field                    `parser:"| @@"`
	Declaration   *Declaration              `parser:"| @@"`
	Foreign       *Foreign                  `parser:"| @@"`
	Enum          *Enum                     `parser:"| @@"`
	// @unsafe { ... } must precede Intrinsic so `@unsafe {` is not read as `@unsafe(`
	UnsafeBlock *UnsafeBlock `parser:"| @@"`
	// Intrinsic must come before declarations
	Intrinsic *Intrinsic `parser:"| @@"`
	// IncDec must come before ExprStmt to avoid i++ being parsed as expression i followed by unexpected ++
	IncDec *IncDec `parser:"| @@"`
	// Loop must come before ExprStmt to avoid `loop {}` being parsed as expression `loop` followed by unexpected `{`
	Loop *Loop `parser:"| @@"`
	// FuncCall and MethodCall are more specific than the generic ExprStmt and
	// must precede it; otherwise ExprStmt (an Expression, which itself contains
	// a FuncCall) swallows every call statement and these alternatives are dead.
	// FuncCall is tried before MethodCall: it captures a single optional
	// `module.`/`Type::` prefix, so `math.add()` stays a module/static call while
	// deeper chains like `self.field.method()` fail FuncCall and fall through to
	// MethodCall.
	FuncCall   *FuncCall   `parser:"| @@"`
	MethodCall *MethodCall `parser:"| @@"`
	ExprStmt   *Expression `parser:"| @@"`
}
