package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

// CScopeInformation holds per-scope C code generation state
type CScopeInformation struct {
	Code          string
	Declarations  []string
	Functions     []string
	Globals       []string
	Types         []string // struct/class type definitions
	TypeDefs      []string // typedef declarations for external types
	CurrentFunc   string
	LocalVars     map[string]string // variable name -> C type
	ChildContexts map[string]*CScopeInformation
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
	info.TypeDefs = make([]string, 0)
	info.LocalVars = make(map[string]string)
	info.ChildContexts = make(map[string]*CScopeInformation)
}

// TypeRefToCType converts a gecko TypeRef to a C type string
func TypeRefToCType(t *tokens.TypeRef, scope *ast.Ast) string {
	if t == nil {
		return "void"
	}

	base := ""

	if t.Array != nil {
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

// IsFuncPointerType checks if a type string is a function pointer type
func IsFuncPointerType(cType string) bool {
	return strings.Contains(cType, "__FUNCPTR__")
}

// FormatFuncPointerDecl formats a function pointer declaration with variable name
func FormatFuncPointerDecl(cType, varName string) string {
	return strings.Replace(cType, "__FUNCPTR__", varName, 1)
}
