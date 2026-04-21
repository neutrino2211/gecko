package cbackend

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

// CurrentTypeState tracks the active TypeState during compilation.
// This enables flow-sensitive type narrowing (e.g., null checks).
var CurrentTypeState *ast.TypeState

// StructDefinition holds a struct definition with its dependencies
type StructDefinition struct {
	Name              string         // The struct name (e.g., "Shell")
	Code              string         // The full typedef struct code
	Dependencies      []string       // All types this struct depends on, including pointers (for ordering)
	ValueDependencies []string       // Non-pointer dependencies only (for cycle detection - these cause infinite size)
	Pos               lexer.Position // Source position for error reporting
}

// CScopeInformation holds per-scope C code generation state
type CScopeInformation struct {
	Code                  string
	Declarations          []string
	Functions             []string
	Globals               []string
	Types                 []string            // struct/class type definitions (deprecated, use StructDefs)
	StructDefs            []*StructDefinition // struct definitions with dependency info
	TypeDefs              []string            // typedef declarations for external types
	Includes              []string            // C header includes from cimport
	CImportLibraries      []string            // Libraries from cimport for pkg-config
	CurrentFunc           string
	CurrentFuncReturnType *tokens.TypeRef // Return type of current function for validation
	LocalVars             map[string]string // variable name -> C type
	ChildContexts         map[string]*CScopeInformation
	TypeState             *ast.TypeState // Flow-sensitive type state for this scope
}

// CValueInformation holds type info for a value
type CValueInformation struct {
	CType     string
	GeckoType *tokens.TypeRef
}

// CScopeData maps scope names to their C code info
type CScopeData map[string]*CScopeInformation

// CValuesMap maps variable names to their C info
type CValuesMap map[string]*CValueInformation

// CBackendImplementation implements BackendCodegenImplementations
type CBackendImplementation struct {
	Backend interfaces.BackendInteface
}

// Primitives maps gecko types to C types
var GeckoToCType = map[string]string{
	"void":   "void",
	"bool":   "int",
	"int":    "int64_t",
	"int8":   "int8_t",
	"int16":  "int16_t",
	"int32":  "int32_t",
	"int64":  "int64_t",
	"uint":   "uint64_t",
	"uint8":  "uint8_t",
	"uint16": "uint16_t",
	"uint32": "uint32_t",
	"uint64": "uint64_t",
	"string": "const char*",
}

// CScopeDataMap holds all scope data
var CScopeDataMap = &CScopeData{}

// CProgramValues holds all value info
var CProgramValues = &CValuesMap{}

// TraitDefinitions stores trait token definitions for default implementations
// Maps trait name (e.g., "Iterator") to its token definition
var TraitDefinitions = make(map[string]*tokens.Trait)

// EnumToCType maps enum names to their mangled C type names
// Separate from GeckoToCType to avoid loadPrimitives overwriting enum ASTs
var EnumToCType = make(map[string]string)

// MethodReturnTypes maps method full names to their return TypeRef
// This preserves generic type arguments that ast.Method.Type (a string) can't hold
var MethodReturnTypes = make(map[string]*tokens.TypeRef)

// CGetScopeInformation retrieves or creates scope info
func CGetScopeInformation(scope *ast.Ast) *CScopeInformation {
	name := scope.GetFullName()

	info, ok := (*CScopeDataMap)[name]

	if !ok {
		info := &CScopeInformation{}
		info.Init()
		(*CScopeDataMap)[name] = info
		return (*CScopeDataMap)[name]
	}

	return info
}

// Init initializes the scope information
func (info *CScopeInformation) Init() {
	info.Declarations = make([]string, 0)
	info.Functions = make([]string, 0)
	info.Globals = make([]string, 0)
	info.Types = make([]string, 0)
	info.StructDefs = make([]*StructDefinition, 0)
	info.TypeDefs = make([]string, 0)
	info.Includes = make([]string, 0)
	info.LocalVars = make(map[string]string)
	info.ChildContexts = make(map[string]*CScopeInformation)
	info.TypeState = ast.NewTypeState()
}

// TypeRefToCType converts a gecko TypeRef to a C type string
func TypeRefToCType(t *tokens.TypeRef, scope *ast.Ast) string {
	if t == nil {
		return "void"
	}

	base := ""

	if t.Size != nil {
		// Fixed-size array: [N]T -> T* (passed as pointer in C)
		base = TypeRefToCType(t.Size.Type, scope) + "*"
	} else if t.Array != nil {
		base = TypeRefToCType(t.Array, scope) + "*"
	} else if t.FuncType != nil {
		// Function pointer type: return_type (*)( param_types... )
		retType := "void"
		if t.FuncType.ReturnType != nil {
			retType = TypeRefToCType(t.FuncType.ReturnType, scope)
		}

		params := ""
		for i, paramType := range t.FuncType.ParamTypes {
			if i > 0 {
				params += ", "
			}
			params += TypeRefToCType(paramType, scope)
		}
		if params == "" {
			params = "void"
		}

		// Return function pointer type with __FUNCPTR__ marker for name placement
		base = retType + " (*__FUNCPTR__)(" + params + ")"
	} else {
		cType, ok := GeckoToCType[t.Type]
		if ok {
			base = cType
		} else if enumType, isEnum := EnumToCType[t.Type]; isEnum {
			base = enumType
		} else if t.Module != "" {
			// Module-qualified type: module.Type
			// Use simple type name (typedef names don't include module prefix)
			if len(t.TypeArgs) > 0 {
				typeArgStrs := make([]string, len(t.TypeArgs))
				for i, typeArg := range t.TypeArgs {
					typeArgStrs[i] = TypeRefToCType(typeArg, scope)
				}
				base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
			} else {
				base = t.Type
			}
		} else {
			// Check if this is a type parameter that should be substituted
			if CurrentMonomorphContext != nil {
				if concreteType, found := CurrentMonomorphContext.GetConcreteTypeForParam(t.Type); found {
					base = concreteType
				} else if len(t.TypeArgs) > 0 {
					// Generic type instantiation
					typeArgStrs := make([]string, len(t.TypeArgs))
					for i, typeArg := range t.TypeArgs {
						typeArgStrs[i] = TypeRefToCType(typeArg, scope)
					}
					base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
				} else {
					base = t.Type
				}
			} else if len(t.TypeArgs) > 0 {
				// Convert type arguments to C types
				typeArgStrs := make([]string, len(t.TypeArgs))
				for i, typeArg := range t.TypeArgs {
					typeArgStrs[i] = TypeRefToCType(typeArg, scope)
				}
				// Request instantiation and get mangled name
				base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
			} else {
				// Unknown type, use as-is (struct or custom type)
				base = t.Type
			}
		}
	}

	// Add volatile qualifier before the pointer
	if t.Volatile {
		base = "volatile " + base
	}

	if t.Pointer {
		base += "*"
	}

	return base
}

// GetMonomorphizedClassName returns the class name to use for lookup.
// For generic types like Raw<uint32>, returns the mangled name like Raw__uint32.
func GetMonomorphizedClassName(t *tokens.TypeRef, scope *ast.Ast) string {
	if t == nil {
		return ""
	}
	if len(t.TypeArgs) > 0 {
		// Generic type - get the mangled name
		typeArgStrs := make([]string, len(t.TypeArgs))
		for i, typeArg := range t.TypeArgs {
			typeArgStrs[i] = TypeRefToCType(typeArg, scope)
		}
		return Generics.RequestClassInstantiation(t.Type, typeArgStrs)
	}
	return t.Type
}

// IsFuncPointerType checks if a type string is a function pointer type
func IsFuncPointerType(cType string) bool {
	return strings.Contains(cType, "__FUNCPTR__")
}

// TopologicalSortStructs sorts struct definitions so dependencies come first.
// Uses Kahn's algorithm for topological sorting.
func TopologicalSortStructs(structs []*StructDefinition) []*StructDefinition {
	if len(structs) == 0 {
		return structs
	}

	// Build maps for quick lookup
	nameToStruct := make(map[string]*StructDefinition)
	for _, s := range structs {
		nameToStruct[s.Name] = s
	}

	// Build in-degree map (count of dependencies not yet processed)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // type -> structs that depend on it

	for _, s := range structs {
		inDegree[s.Name] = 0
	}

	for _, s := range structs {
		for _, dep := range s.Dependencies {
			if _, exists := nameToStruct[dep]; exists {
				inDegree[s.Name]++
				dependents[dep] = append(dependents[dep], s.Name)
			}
		}
	}

	// Start with structs that have no dependencies
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]*StructDefinition, 0, len(structs))

	for len(queue) > 0 {
		// Pop from queue
		name := queue[0]
		queue = queue[1:]

		if s, ok := nameToStruct[name]; ok {
			result = append(result, s)
		}

		// Reduce in-degree for dependents
		for _, dependent := range dependents[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// If we couldn't process all structs, there's a cycle - return original order
	if len(result) < len(structs) {
		return structs
	}

	return result
}

// CircularDependency represents a cycle in type dependencies
type CircularDependency struct {
	Types []*StructDefinition // Types involved in the cycle
}

// DetectCircularValueDependencies checks for cycles in non-pointer (value) dependencies.
// These cycles cause infinite struct sizes and are compile errors.
// Returns a list of cycles found, empty if no cycles.
func DetectCircularValueDependencies(structs []*StructDefinition) []CircularDependency {
	if len(structs) == 0 {
		return nil
	}

	// Build maps for quick lookup
	nameToStruct := make(map[string]*StructDefinition)
	for _, s := range structs {
		nameToStruct[s.Name] = s
	}

	// Build adjacency list from VALUE dependencies only
	adj := make(map[string][]string)
	for _, s := range structs {
		for _, dep := range s.ValueDependencies {
			if _, exists := nameToStruct[dep]; exists {
				adj[s.Name] = append(adj[s.Name], dep)
			}
		}
	}

	// Track visited state for cycle detection
	// 0 = unvisited, 1 = in current path (gray), 2 = fully visited (black)
	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles []CircularDependency

	// DFS to find cycles
	var dfs func(node string) bool
	dfs = func(node string) bool {
		color[node] = 1 // Mark as being visited

		for _, neighbor := range adj[node] {
			if color[neighbor] == 1 {
				// Found a cycle - reconstruct it
				cycle := []*StructDefinition{nameToStruct[neighbor]}
				curr := node
				for curr != neighbor {
					cycle = append([]*StructDefinition{nameToStruct[curr]}, cycle...)
					curr = parent[curr]
				}
				cycles = append(cycles, CircularDependency{Types: cycle})
				return true
			}
			if color[neighbor] == 0 {
				parent[neighbor] = node
				if dfs(neighbor) {
					return true
				}
			}
		}

		color[node] = 2 // Mark as fully visited
		return false
	}

	// Run DFS from each unvisited node
	for _, s := range structs {
		if color[s.Name] == 0 {
			dfs(s.Name)
		}
	}

	return cycles
}

// FormatFuncPointerDecl formats a function pointer declaration with variable name
func FormatFuncPointerDecl(cType, varName string) string {
	return strings.Replace(cType, "__FUNCPTR__", varName, 1)
}

// GetScopedTypeName returns the scoped C type name for a module-qualified type.
// This is the single source of truth for type name mangling with module prefixes.
// Examples:
//   - GetScopedTypeName("geometry", "Point") -> "geometry__Point"
//   - GetScopedTypeName("", "Point") -> "Point"
//   - GetScopedTypeName("std.collections", "Vec") -> "std__collections__Vec"
func GetScopedTypeName(module string, typeName string) string {
	if module == "" {
		return typeName
	}
	// Replace dots with double underscores for nested modules
	modulePrefix := strings.ReplaceAll(module, ".", "__")
	return modulePrefix + "__" + typeName
}

// ResolveClassFromTypeRef resolves a TypeRef to a class AST, handling module qualification.
// For module-qualified types (e.g., geometry.Point), it searches the child scope.
// Returns the class AST and the scoped C name for the type.
func ResolveClassFromTypeRef(typeRef *tokens.TypeRef, scope *ast.Ast) (*ast.Ast, string) {
	if typeRef == nil {
		return nil, ""
	}

	rootScope := scope.GetRoot()
	typeName := typeRef.Type
	scopedName := GetScopedTypeName(typeRef.Module, typeName)

	// Handle generic types first
	if len(typeRef.TypeArgs) > 0 {
		typeArgStrs := make([]string, len(typeRef.TypeArgs))
		for i, typeArg := range typeRef.TypeArgs {
			typeArgStrs[i] = TypeRefToCType(typeArg, scope)
		}
		scopedName = Generics.RequestClassInstantiation(scopedName, typeArgStrs)
	}

	// For module-qualified types, search the child scope first
	if typeRef.Module != "" {
		if child, ok := rootScope.Children[typeRef.Module]; ok {
			if classOpt := child.ResolveClass(typeName); !classOpt.IsNil() {
				return classOpt.Unwrap(), scopedName
			}
		}
	}

	// Search in root scope
	if classOpt := rootScope.ResolveClass(typeName); !classOpt.IsNil() {
		return classOpt.Unwrap(), scopedName
	}

	// Search imported modules
	for _, child := range rootScope.Children {
		if classOpt := child.ResolveClass(typeName); !classOpt.IsNil() {
			return classOpt.Unwrap(), scopedName
		}
	}

	return nil, scopedName
}
