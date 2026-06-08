// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/neutrino2211/gecko/tokens"
)

// GenericInstantiation tracks a specific instantiation of a generic type/function
type GenericInstantiation struct {
	Name         string   // Original name (e.g., "Box")
	TypeArgs     []string // Concrete types (e.g., ["int64_t"])
	FullName     string   // Mangled name (e.g., "Box__int64_t")
	OriginModule string   // Module where the generic was defined (e.g., "slice")
}

// GenericRegistry tracks all generic definitions and their instantiations
type GenericRegistry struct {
	// Generic class definitions: name -> Class token
	GenericClasses map[string]*tokens.Class
	// Origin module for generic classes: name -> module name
	GenericClassOrigins map[string]string
	// Generic method definitions: name -> Method token
	GenericMethods map[string]*tokens.Method
	// Instantiations that need to be generated
	ClassInstantiations  []*GenericInstantiation
	MethodInstantiations []*GenericInstantiation
	// Already generated instantiations (to avoid duplicates)
	GeneratedClasses map[string]bool
	GeneratedMethods map[string]bool
}

// MonomorphContext tracks type parameter mappings during monomorphization
type MonomorphContext struct {
	// Maps type parameter name to concrete type
	TypeMap map[string]string
	// Maps type parameter name to trait constraints (supports multiple: T is A & B)
	Constraints map[string][]string
}

var Generics = &GenericRegistry{
	GenericClasses:       make(map[string]*tokens.Class),
	GenericClassOrigins:  make(map[string]string),
	GenericMethods:       make(map[string]*tokens.Method),
	ClassInstantiations:  make([]*GenericInstantiation, 0),
	MethodInstantiations: make([]*GenericInstantiation, 0),
	GeneratedClasses:     make(map[string]bool),
	GeneratedMethods:     make(map[string]bool),
}

// NewGenericRegistry creates a fresh GenericRegistry.
func NewGenericRegistry() *GenericRegistry {
	return &GenericRegistry{
		GenericClasses:       make(map[string]*tokens.Class),
		GenericClassOrigins:  make(map[string]string),
		GenericMethods:       make(map[string]*tokens.Method),
		ClassInstantiations:  make([]*GenericInstantiation, 0),
		MethodInstantiations: make([]*GenericInstantiation, 0),
		GeneratedClasses:     make(map[string]bool),
		GeneratedMethods:     make(map[string]bool),
	}
}

// CurrentMonomorphContext is set during monomorphization to track type mappings
var CurrentMonomorphContext *MonomorphContext

// InitTypeParameterChecker sets up the type parameter checker in the tokens package.
// Must be called during backend initialization, before compilation begins.
func InitTypeParameterChecker() {
	tokens.IsTypeParameter = func(name string) bool {
		if CurrentMonomorphContext == nil {
			return false
		}
		_, isTypeParam := CurrentMonomorphContext.GetConcreteTypeForParam(name)
		return isTypeParam
	}
}

// BuildMonomorphContext creates a context for monomorphization
func BuildMonomorphContext(typeParams []*tokens.TypeParam, typeArgs []string) *MonomorphContext {
	ctx := &MonomorphContext{
		TypeMap:     make(map[string]string),
		Constraints: make(map[string][]string),
	}
	for i, param := range typeParams {
		if i < len(typeArgs) {
			ctx.TypeMap[param.Name] = typeArgs[i]
			if traits := param.AllTraits(); len(traits) > 0 {
				ctx.Constraints[param.Name] = traits
			}
		}
	}
	return ctx
}

// GetConcreteTypeForParam returns the concrete type for a type parameter
func (ctx *MonomorphContext) GetConcreteTypeForParam(paramName string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	concreteType, ok := ctx.TypeMap[paramName]
	return concreteType, ok
}

// GetTraitConstraints returns all trait constraints for a type parameter
func (ctx *MonomorphContext) GetTraitConstraints(paramName string) ([]string, bool) {
	if ctx == nil {
		return nil, false
	}
	traits, ok := ctx.Constraints[paramName]
	return traits, ok && len(traits) > 0
}

// GetTraitConstraint returns the first trait constraint for backwards compatibility
func (ctx *MonomorphContext) GetTraitConstraint(paramName string) (string, bool) {
	traits, ok := ctx.GetTraitConstraints(paramName)
	if !ok || len(traits) == 0 {
		return "", false
	}
	return traits[0], true
}

// FindTraitWithMethod searches the type parameter's trait constraints to find which trait provides a method.
// Returns the trait name if found, empty string if not found.
func (ctx *MonomorphContext) FindTraitWithMethod(paramName string, methodName string) string {
	traits, ok := ctx.GetTraitConstraints(paramName)
	if !ok {
		return ""
	}

	for _, traitName := range traits {
		if TraitDefinesMethod(traitName, methodName) {
			return traitName
		}
	}
	return ""
}

// RegisterGenericClass registers a generic class definition with its origin module
func (g *GenericRegistry) RegisterGenericClass(name string, class *tokens.Class, originModule string) {
	g.GenericClasses[name] = class
	g.GenericClassOrigins[name] = originModule
}

// RegisterGenericMethod registers a generic method definition
func (g *GenericRegistry) RegisterGenericMethod(name string, method *tokens.Method) {
	g.GenericMethods[name] = method
}

// RequestClassInstantiation requests a specific instantiation of a generic class
func (g *GenericRegistry) RequestClassInstantiation(name string, typeArgs []string) string {
	fullName := mangleName(name, typeArgs)

	if g.GeneratedClasses[fullName] {
		return fullName
	}

	originModule := g.GenericClassOrigins[name]

	g.ClassInstantiations = append(g.ClassInstantiations, &GenericInstantiation{
		Name:         name,
		TypeArgs:     typeArgs,
		FullName:     fullName,
		OriginModule: originModule,
	})
	g.GeneratedClasses[fullName] = true

	return fullName
}

// RequestMethodInstantiation requests a specific instantiation of a generic method
func (g *GenericRegistry) RequestMethodInstantiation(name string, typeArgs []string) string {
	fullName := mangleName(name, typeArgs)

	if g.GeneratedMethods[fullName] {
		return fullName
	}

	g.MethodInstantiations = append(g.MethodInstantiations, &GenericInstantiation{
		Name:     name,
		TypeArgs: typeArgs,
		FullName: fullName,
	})
	g.GeneratedMethods[fullName] = true

	return fullName
}

// IsGenericClass checks if a class is generic
func (g *GenericRegistry) IsGenericClass(name string) bool {
	_, ok := g.GenericClasses[name]
	return ok
}

// IsGenericMethod checks if a method is generic
func (g *GenericRegistry) IsGenericMethod(name string) bool {
	_, ok := g.GenericMethods[name]
	return ok
}

// mangleName creates a mangled name for a generic instantiation
func mangleName(name string, typeArgs []string) string {
	if len(typeArgs) == 0 {
		return name
	}
	mangled := make([]string, len(typeArgs))
	for i, typeArg := range typeArgs {
		mangled[i] = MangleTypeArgForIdentifier(typeArg)
	}
	return name + "__" + strings.Join(mangled, "__")
}

// MangleTypeArgForIdentifier converts a concrete type string (possibly C-like, e.g. "const char*")
// into an identifier-safe fragment used in generated symbol names.
func MangleTypeArgForIdentifier(typeArg string) string {
	raw := strings.TrimSpace(typeArg)
	if raw == "" {
		return "void"
	}

	var b strings.Builder
	prevUnderscore := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}

		repl := "_"
		if r == '*' {
			repl = "_ptr"
		}

		if repl == "_" && prevUnderscore {
			continue
		}
		b.WriteString(repl)
		prevUnderscore = strings.HasSuffix(repl, "_")
	}

	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "void"
	}
	if unicode.IsDigit(rune(out[0])) {
		out = "t_" + out
	}
	return out
}

// SubstituteTypeParams replaces type parameters with concrete types
// Uses word boundary matching to avoid replacing partial matches
func SubstituteTypeParams(typeStr string, typeParams []*tokens.TypeParam, typeArgs []string) string {
	result := typeStr
	for i, param := range typeParams {
		if i < len(typeArgs) {
			// Use word boundary regex to only replace whole identifiers
			pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(param.Name) + `\b`)
			result = pattern.ReplaceAllString(result, typeArgs[i])
		}
	}
	return result
}

// ResetGenerics clears the generic registry (for fresh compilation)
func ResetGenerics() {
	Generics = &GenericRegistry{
		GenericClasses:       make(map[string]*tokens.Class),
		GenericClassOrigins:  make(map[string]string),
		GenericMethods:       make(map[string]*tokens.Method),
		ClassInstantiations:  make([]*GenericInstantiation, 0),
		MethodInstantiations: make([]*GenericInstantiation, 0),
		GeneratedClasses:     make(map[string]bool),
		GeneratedMethods:     make(map[string]bool),
	}
}
