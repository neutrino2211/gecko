package ast

import (
	"strings"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/go-option"
)

// LazyTypeResolver is called when a type cannot be found in the scope hierarchy.
// It allows directory imports to be resolved on-demand.
type LazyTypeResolver func(typeName string) (*Ast, bool)

// LazyMethodResolver is called when a method cannot be found in the scope hierarchy.
// It allows directory imports to be resolved on-demand.
type LazyMethodResolver func(methodName string) (*Method, bool)

// LazyModuleTypeResolver is called for module-qualified types (e.g., shapes.Circle)
// when the module isn't found in Children. Returns the resolved class AST.
type LazyModuleTypeResolver func(moduleName string, typeName string) (*Ast, bool)

// TypeSuggestionProvider returns import suggestions for an unresolved type.
// The return value is appended to the type error message.
type TypeSuggestionProvider func(typeName string) string

type Ast struct {
	Scope            string
	Imports          []string
	Methods          map[string]*Method
	Variables        map[string]Variable
	Classes          map[string]*Ast
	Children         map[string]*Ast // Imported modules
	Traits           map[string]*[]*Method
	Parent           *Ast
	ErrorScope       *errors.ErrorScope
	Config           *config.CompileCfg
	IsPacked             bool                   // Set to true if class has @packed attribute
	Visibility           string                 // "public", "private", "protected", or "" (default private)
	OriginModule         string                 // Package name where this symbol was defined
	SourceFile           string                 // Source file path where this symbol was defined
	IsImportedModule     bool                   // True if this scope is an imported module (not the main file)
	LazyResolver           LazyTypeResolver       // Called to resolve types from directory imports
	LazyMethodResolver     LazyMethodResolver     // Called to resolve methods from directory imports
	LazyModuleTypeResolver LazyModuleTypeResolver // Called for module-qualified types (e.g., shapes.Circle)
	SuggestionProvider     TypeSuggestionProvider // Called to get import suggestions for unresolved types
}

func (a *Ast) Init(errorScope *errors.ErrorScope) {
	a.Methods = make(map[string]*Method)
	a.Variables = make(map[string]Variable)
	a.Classes = make(map[string]*Ast)
	a.Children = make(map[string]*Ast)
	a.Traits = make(map[string]*[]*Method)
	a.Imports = []string{}
	a.ErrorScope = errorScope
	a.Config = &config.CompileCfg{}
}

func (a *Ast) FullScopeName() string {
	r := a.Scope
	parent := a.Parent

	for parent != nil {
		r = parent.Scope + "." + r
		parent = parent.Parent
	}

	return r
}

func (a *Ast) GetFullName() string {
	cString := ""

	if a.Parent == nil {
		cString = a.Scope
	} else {
		cString = strings.ReplaceAll(a.FullScopeName(), ".", "__")
	}

	return cString
}

// GetRoot returns the root AST node by traversing up the parent chain
func (a *Ast) GetRoot() *Ast {
	root := a
	for root.Parent != nil {
		root = root.Parent
	}
	return root
}

func (a *Ast) ResolveSymbolAsVariable(symbol string) *option.Optional[*Variable] {
	scope := a
	symbolVariable, ok := scope.Variables[symbol]

	for !ok {
		if scope.Parent == nil {
			return option.None[*Variable]()
		}

		scope = scope.Parent
		symbolVariable, ok = scope.Variables[symbol]
	}

	return option.Some(&symbolVariable)
}

func (a *Ast) ResolveMethod(mth string) *option.Optional[*Method] {
	scope := a
	mthMethod, ok := scope.Methods[mth]

	for !ok {
		if scope.Parent == nil {
			// Try lazy resolution from directory imports
			root := a.GetRoot()
			if root.LazyMethodResolver != nil {
				if resolved, found := root.LazyMethodResolver(mth); found {
					// Cache in root scope for future lookups
					root.Methods[mth] = resolved
					return option.Some(resolved)
				}
			}
			return option.None[*Method]()
		}

		scope = scope.Parent
		mthMethod, ok = scope.Methods[mth]
	}

	return option.Some(mthMethod)
}

func (a *Ast) ResolveClass(class string) *option.Optional[*Ast] {
	scope := a
	clsClass, ok := scope.Classes[class]

	for !ok {
		if scope.Parent == nil {
			// Try lazy resolution from directory imports
			root := a.GetRoot()
			// DEBUG: Check if lazy resolver is set
			if root.LazyResolver != nil {
				if resolved, found := root.LazyResolver(class); found {
					// Cache in root scope for future lookups
					root.Classes[class] = resolved
					return option.Some(resolved)
				}
			}
			return option.None[*Ast]()
		}

		scope = scope.Parent
		clsClass, ok = scope.Classes[class]
	}

	return option.Some(clsClass)
}

// ResolveClassWithLazyFallback is like ResolveClass but accepts an external lazy resolver.
// This is useful when the AST structure doesn't have a parent chain back to the root.
func (a *Ast) ResolveClassWithLazyFallback(class string, lazyResolver LazyTypeResolver) *option.Optional[*Ast] {
	// First try normal resolution
	result := a.ResolveClass(class)
	if !result.IsNil() {
		return result
	}

	// Fall back to lazy resolver if provided
	if lazyResolver != nil {
		if resolved, found := lazyResolver(class); found {
			// Cache in root scope for future lookups
			root := a.GetRoot()
			root.Classes[class] = resolved
			return option.Some(resolved)
		}
	}

	return option.None[*Ast]()
}

func (a *Ast) ResolveTrait(trait string) *option.Optional[*[]*Method] {
	scope := a
	trTrait, ok := scope.Traits[trait]

	for !ok {
		if scope.Parent == nil {
			return option.None[*[]*Method]()
		}

		scope = scope.Parent
		trTrait, ok = scope.Traits[trait]
	}

	return option.Some(trTrait)
}

func (a *Ast) ToCString() string {
	r := ""
	// solve
	return r
}

// IsPublic returns true if this symbol is accessible from anywhere
func (a *Ast) IsPublic() bool {
	return a.Visibility == "public" || a.Visibility == "external"
}

// IsProtected returns true if this symbol is accessible within the same package
func (a *Ast) IsProtected() bool {
	return a.Visibility == "protected"
}

// IsPrivate returns true if this symbol is only accessible within the same file
func (a *Ast) IsPrivate() bool {
	return a.Visibility == "private" || a.Visibility == ""
}

// GetOriginModule returns the package where this symbol was defined
func (a *Ast) GetOriginModule() string {
	if a.OriginModule != "" {
		return a.OriginModule
	}
	// Fall back to root scope name
	return a.GetRoot().Scope
}

// GetSourceFile returns the source file where this symbol was defined
func (a *Ast) GetSourceFile() string {
	if a.SourceFile != "" {
		return a.SourceFile
	}
	// Fall back to parent's source file
	if a.Parent != nil {
		return a.Parent.GetSourceFile()
	}
	return ""
}

// IsSameFile checks if two AST nodes are from the same source file
func (a *Ast) IsSameFile(other *Ast) bool {
	if a == nil || other == nil {
		return false
	}
	aFile := a.GetSourceFile()
	otherFile := other.GetSourceFile()
	// If either file is empty, fall back to module comparison
	if aFile == "" || otherFile == "" {
		return a.IsSamePackage(other)
	}
	return aFile == otherFile
}

// IsSamePackage checks if two AST nodes are from the same package
func (a *Ast) IsSamePackage(other *Ast) bool {
	if a == nil || other == nil {
		return false
	}
	return a.GetOriginModule() == other.GetOriginModule()
}

// IsSameModule checks if two AST nodes are from the same module (alias for IsSamePackage)
func (a *Ast) IsSameModule(other *Ast) bool {
	return a.IsSamePackage(other)
}

// CheckVisibility validates that a symbol can be accessed from the given scope.
// Returns an error message if access is denied, empty string if allowed.
// Visibility levels:
//   - private (default): same file only
//   - protected: same package (any file in package)
//   - public: accessible from anywhere
func (a *Ast) CheckVisibility(fromScope *Ast, symbolName string) string {
	// Public symbols are always accessible
	if a.IsPublic() {
		return ""
	}

	// Protected symbols are accessible within the same package
	if a.IsProtected() {
		if a.IsSamePackage(fromScope) {
			return ""
		}
		return "'" + symbolName + "' is protected and can only be accessed within package '" + a.GetOriginModule() + "'"
	}

	// Private symbols (default) are only accessible within the same file
	if a.IsSameFile(fromScope) {
		return ""
	}

	visibility := a.Visibility
	if visibility == "" {
		visibility = "private (default)"
	}
	return "'" + symbolName + "' is " + visibility + " and can only be accessed within the same file"
}
