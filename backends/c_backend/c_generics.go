package cbackend

import (
	"regexp"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

// GenericInstantiation tracks a specific instantiation of a generic type/function
type GenericInstantiation struct {
	Name     string            // Original name (e.g., "Box")
	TypeArgs []string          // Concrete types (e.g., ["int64_t"])
	FullName string            // Mangled name (e.g., "Box__int64_t")
}

// GenericRegistry tracks all generic definitions and their instantiations
type GenericRegistry struct {
	// Generic class definitions: name -> Class token
	GenericClasses map[string]*tokens.Class
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
	// Maps type parameter name to trait constraint (if any)
	Constraints map[string]string
}

var Generics = &GenericRegistry{
	GenericClasses:       make(map[string]*tokens.Class),
	GenericMethods:       make(map[string]*tokens.Method),
	ClassInstantiations:  make([]*GenericInstantiation, 0),
	MethodInstantiations: make([]*GenericInstantiation, 0),
	GeneratedClasses:     make(map[string]bool),
	GeneratedMethods:     make(map[string]bool),
}

// CurrentMonomorphContext is set during monomorphization to track type mappings
var CurrentMonomorphContext *MonomorphContext

// BuildMonomorphContext creates a context for monomorphization
func BuildMonomorphContext(typeParams []*tokens.TypeParam, typeArgs []string) *MonomorphContext {
	ctx := &MonomorphContext{
		TypeMap:     make(map[string]string),
		Constraints: make(map[string]string),
	}
	for i, param := range typeParams {
		if i < len(typeArgs) {
			ctx.TypeMap[param.Name] = typeArgs[i]
			if param.Trait != "" {
				ctx.Constraints[param.Name] = param.Trait
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

// GetTraitConstraint returns the trait constraint for a type parameter
func (ctx *MonomorphContext) GetTraitConstraint(paramName string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	trait, ok := ctx.Constraints[paramName]
	return trait, ok
}

// RegisterGenericClass registers a generic class definition
func (g *GenericRegistry) RegisterGenericClass(name string, class *tokens.Class) {
	g.GenericClasses[name] = class
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

	g.ClassInstantiations = append(g.ClassInstantiations, &GenericInstantiation{
		Name:     name,
		TypeArgs: typeArgs,
		FullName: fullName,
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
	return name + "__" + strings.Join(typeArgs, "__")
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
		GenericMethods:       make(map[string]*tokens.Method),
		ClassInstantiations:  make([]*GenericInstantiation, 0),
		MethodInstantiations: make([]*GenericInstantiation, 0),
		GeneratedClasses:     make(map[string]bool),
		GeneratedMethods:     make(map[string]bool),
	}
}
