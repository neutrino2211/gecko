// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

// Generic type parameters

type TypeParam struct {
	baseToken
	Name   string   `parser:"@Ident"`
	Trait  string   `parser:"[ 'is' @Ident"` // First trait (kept for backwards compatibility)
	Traits []string `parser:"    { '&' @Ident } ]"`
}

// AllTraits returns all trait constraints (Trait + Traits)
func (t *TypeParam) AllTraits() []string {
	if t.Trait == "" {
		return nil
	}
	return append([]string{t.Trait}, t.Traits...)
}

// WhereClause is a trailing generic constraint list:
// `func f<T, U>(...) where T is Eq & Ord, U is Show { ... }`.
type WhereClause struct {
	baseToken
	Constraints []*WhereConstraint `parser:"'where' @@ { ',' @@ }"`
}

// WhereConstraint constrains one type parameter: `T is Eq & Ord`.
type WhereConstraint struct {
	baseToken
	Name   string   `parser:"@Ident 'is'"`
	Trait  string   `parser:"@Ident"`
	Traits []string `parser:"{ '&' @Ident }"`
}

// AllTraits returns every trait named in the constraint.
func (c *WhereConstraint) AllTraits() []string {
	if c == nil || c.Trait == "" {
		return nil
	}
	return append([]string{c.Trait}, c.Traits...)
}

// HasTrait reports whether name is already one of the parameter's constraints.
func (t *TypeParam) HasTrait(name string) bool {
	if t == nil || name == "" {
		return false
	}
	if t.Trait == name {
		return true
	}
	for _, tr := range t.Traits {
		if tr == name {
			return true
		}
	}
	return false
}

// ApplyWhereClause merges where-clause constraints into the matching type
// parameters, so consumers can keep reading constraints via TypeParam.AllTraits.
// The operation is idempotent, which makes it safe to run more than once.
func ApplyWhereClause(typeParams []*TypeParam, where *WhereClause) {
	if where == nil {
		return
	}
	byName := make(map[string]*TypeParam, len(typeParams))
	for _, tp := range typeParams {
		if tp != nil {
			byName[tp.Name] = tp
		}
	}
	for _, c := range where.Constraints {
		if c == nil {
			continue
		}
		tp, ok := byName[c.Name]
		if !ok {
			continue
		}
		for _, trait := range c.AllTraits() {
			if tp.HasTrait(trait) {
				continue
			}
			if tp.Trait == "" {
				tp.Trait = trait
			} else {
				tp.Traits = append(tp.Traits, trait)
			}
		}
	}
}

// NormalizeWhereClauses folds every `where` clause in a parsed file into the
// matching type parameters. Call once after parsing so all later phases see a
// single, uniform source of generic constraints.
func NormalizeWhereClauses(file *File) {
	if file == nil {
		return
	}
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Method != nil {
			ApplyWhereClause(entry.Method.TypeParams, entry.Method.Where)
		}
		if entry.Class != nil {
			ApplyWhereClause(entry.Class.TypeParams, entry.Class.Where)
			for _, field := range entry.Class.Fields {
				if field != nil && field.Method != nil {
					ApplyWhereClause(field.Method.TypeParams, field.Method.Where)
				}
			}
		}
	}
}

// Class tokens

type Class struct {
	baseToken
	DocComment      []string           `parser:"{ @DocComment }"`
	Attributes      []*Attribute       `parser:"{ @@ }"`
	Visibility      string             `parser:"[ @'private' | @'public' | @'protected' ]"`
	ExternalName    string             `parser:"[ 'external' @String ]"`
	Name            string             `parser:"'class' @Ident"`
	TypeParams      []*TypeParam       `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Where           *WhereClause       `parser:"[ @@ ]"`
	Fields          []*ClassBlockField `parser:"'{' { @@ } '}'"`
	Implementations []*Implementation
}

type ClassBlockField struct {
	baseToken
	Field  *Field  `parser:"@@"`
	Method *Method `parser:"| @@"`
}

type ClassField struct {
	baseToken
	Field
}

type ClassMethod struct {
	baseToken
	Method
}

// Misc TODO: Sort

type Enum struct {
	baseToken
	Name  string   `parser:"'enum' @Ident"`
	Cases []string `parser:"'{' { @Ident } '}'"`
}

type Trait struct {
	baseToken
	DocComment []string               `parser:"{ @DocComment }"`
	Attributes []*Attribute           `parser:"{ @@ }"`
	Visibility string                 `parser:"[ @'private' | @'public' | @'protected' ]"`
	Name       string                 `parser:"'trait' @Ident"`
	TypeParams []*TypeParam           `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Parent     string                 `parser:"[ ':' @Ident ]"`
	Fields     []*ImplementationField `parser:"'{' { @@ } '}'"`
}

// AllParents returns all inherited trait names.
func (t *Trait) AllParents() []string {
	if t == nil || t.Parent == "" {
		return nil
	}
	return []string{t.Parent}
}

type Implementation struct {
	baseToken
	DocComment []string        `parser:"{ @DocComment }"`
	Visibility string          `parser:"[ @'private' | @'public' | @'protected' ]"`
	Default    bool            `parser:"[ @'default' ]"`
	Attributes []*Attribute    `parser:"{ @@ }"`
	Generic    *GenericImpl    `parser:"'impl' ( @@"`
	NonGeneric *NonGenericImpl `parser:"       | @@ )"`
}

// GenericImpl handles impl<T> Trait<Args> for Class<Args>
type GenericImpl struct {
	baseToken
	TypeParams  []*TypeParam           `parser:"'<' @@ { ',' @@ } '>'"`
	Name        string                 `parser:"@Ident"`
	TypeArgs    []*TypeRef             `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	For         string                 `parser:"[ 'for' @Ident"`
	ForTypeArgs []*TypeRef             `parser:"  [ '<' @@ { ',' @@ } '>' ] ]"`
	Fields      []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

// NonGenericImpl handles impl Trait<Args> for Class<Args> (no impl-level type params)
type NonGenericImpl struct {
	baseToken
	Name        string                 `parser:"@Ident"`
	TypeArgs    []*TypeRef             `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	For         string                 `parser:"[ 'for' @Ident"`
	ForTypeArgs []*TypeRef             `parser:"  [ '<' @@ { ',' @@ } '>' ] ]"`
	Fields      []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

// Accessor methods for Implementation to work with either Generic or NonGeneric

func (i *Implementation) GetTypeParams() []*TypeParam {
	if i.Generic != nil {
		return i.Generic.TypeParams
	}
	return nil
}

func (i *Implementation) GetName() string {
	if i.Generic != nil {
		return i.Generic.Name
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.Name
	}
	return ""
}

func (i *Implementation) GetTypeArgs() []*TypeRef {
	if i.Generic != nil {
		return i.Generic.TypeArgs
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.TypeArgs
	}
	return nil
}

func (i *Implementation) GetFor() string {
	if i.Generic != nil {
		return i.Generic.For
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.For
	}
	return ""
}

func (i *Implementation) GetForTypeArgs() []*TypeRef {
	if i.Generic != nil {
		return i.Generic.ForTypeArgs
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.ForTypeArgs
	}
	return nil
}

// GetAttributes returns the compile-time attributes attached to the impl
// (e.g. @attach_handler), used to register unsafe-handler coverage globally.
func (i *Implementation) GetAttributes() []*Attribute {
	return i.Attributes
}

func (i *Implementation) GetFields() []*ImplementationField {
	if i.Generic != nil {
		return i.Generic.Fields
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.Fields
	}
	return nil
}

type Field struct {
	baseToken
	DocComment []string     `parser:"{ @DocComment }"`
	Attributes []*Attribute `parser:"{ @@ }"`
	Visibility string       `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Mutability string       `parser:"(@'let' | @'const')"`
	Name       string       `parser:"@Ident"`
	Type       *TypeRef     `parser:"[ ':' @@ ]"`
	Value      *Expression  `parser:"[ '=' @@ ]"`
}

// DestructuringDeclaration binds struct fields to local variables in one
// statement: `let Point { x, y } = p`. Shorthand `x` binds field x to local x;
// `x = a` binds field x to local a.
type DestructuringDeclaration struct {
	baseToken
	Mutability string                `parser:"(@'let' | @'const')"`
	TypeName   string                `parser:"@Ident"`
	TypeArgs   []*TypeRef            `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Bindings   []*DestructureBinding `parser:"'{' @@ { ',' @@ } '}'"`
	Value      *Expression           `parser:"'=' @@"`
}

// DestructureBinding is one field binding in a DestructuringDeclaration.
type DestructureBinding struct {
	baseToken
	Field string `parser:"@Ident"`
	Alias string `parser:"[ '=' @Ident ]"`
}

// Target returns the local variable name a binding introduces.
func (b *DestructureBinding) Target() string {
	if b == nil {
		return ""
	}
	if b.Alias != "" {
		return b.Alias
	}
	return b.Field
}

type Assignment struct {
	baseToken
	Global bool        `parser:"[ @'global' ]"`
	Name   string      `parser:"@Ident"`
	Field  string      `parser:"[ '.' @Ident ]"`
	Index  *Expression `parser:"[ '[' @@ ']' ]"`
	Op     string      `parser:"@( '=' | '+=' | '-=' | '*=' | '/=' | '&=' | '|=' | '^=' | '%=' )"`
	Value  *Expression `parser:"@@"`
}

type Declaration struct {
	baseToken
	Method       *Method       `parser:"( 'declare' @@ "`
	Field        *Field        `parser:"| 'declare' @@"`
	ExternalType *ExternalType `parser:"| 'declare' @@)"`
}

// ExternalType declares an opaque type that exists in C land
// Example: declare external type FILE
type ExternalType struct {
	baseToken
	Name string `parser:"'external' 'type' @Ident"`
}

type ImplementationField struct {
	baseToken
	DocComment []string `parser:"{ @DocComment }"`
	Visibility string   `parser:"[ @'private' | @'public' | @'protected' ]"`
	Name       string   `parser:"'func' @Ident"`
	Arguments  []*Value `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
	Type       *TypeRef `parser:"[ ':' @@ ]"`
	Value      []*Entry `parser:"[ '{' @@* '}' ]"`
}

type Method struct {
	baseToken
	DocComment []string     `parser:"{ @DocComment }"`
	Attributes []*Attribute `parser:"{ @@ }"`
	Visibility string       `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Variardic  bool         `parser:"[ @'variardic' ]"`
	Name       string       `parser:"'func' @Ident"`
	TypeParams []*TypeParam `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Arguments  []*Value     `parser:"'(' [ @@ { ',' @@ } ]"`
	Variadic   bool         `parser:"[ ',' @'...' | @'...' ] ')'"`
	Type       *TypeRef     `parser:"[ ':' @@ ]"`
	Throws     *TypeRef     `parser:"[ 'throws' @@ ]"`
	Where      *WhereClause `parser:"[ @@ ]"`
	LinkName   string
	Value      []*Entry `parser:"[ '{' @@* '}' ]"`
}

func (m *Method) IsVariadic() bool {
	if m == nil {
		return false
	}
	return m.Variadic || m.Variardic
}

type Value struct {
	baseToken
	Variadic bool        `parser:"[ @'...' ]"`
	Name     string      `parser:"@Ident"`
	Out      bool        `parser:"[ ':' [ @'out' ]"`
	Type     *TypeRef    `parser:"@@ ]"`
	Default  *Expression `parser:"[ '=' @@ ]"`
}

type Argument struct {
	baseToken
	Name    string      `parser:"[ @Ident ':' ]"`
	Out     bool        `parser:"[ @'out' ]"`
	Value   *Expression `parser:"( @@"`
	SubCall *FuncCall   `parser:"| @@)"`
}

type SizeDef struct {
	baseToken
	Size string   `parser:"'[' @Number ']'"`
	Type *TypeRef `parser:"@@"`
}

type TypeRef struct {
	baseToken
	Array    *TypeRef   `parser:"( '[' @@ ']'"`
	Size     *SizeDef   `parser:" | @@"`
	FuncType *FuncType  `parser:" | @@"`
	Module   string     `parser:" | ( @Ident '.'"`
	Type     string     `parser:"     @Ident ) | @Ident)"`
	TypeArgs []*TypeRef `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Trait    string     `parser:"[ 'is' @Ident ]"`
	// Const is the readonly type qualifier: `T readonly` / `T readonly*`.
	// Distinct from binding `const` (Mutability); strips only via as! in @unsafe.
	Const    bool `parser:"[ @'readonly' ]"`
	Volatile bool `parser:"[ @'volatile' ]"`
	Pointer  bool `parser:"[ @'*']"`
	NonNull  bool `parser:"[ @'!' ]"` // T*! = non-nullable pointer
}
