// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

// CurrentTypeState tracks the active TypeState during compilation.
// This enables flow-sensitive type narrowing (e.g., null checks).
var CurrentTypeState *ast.TypeState

// CurrentSelfType tracks the current class type for resolving `Self` in method signatures.
var CurrentSelfType string

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
	UnsafeSetup           *unsafeSetupContext
	LocalVarOrder         []string
	ChildSequence         int
	LexicalBlock          bool
	FunctionBoundary      bool
	LoopBoundary          bool
	PreparingDefer        bool
	Code                  string
	Declarations          []string
	Functions             []string
	Globals               []string
	Types                 []string            // struct/class type definitions (deprecated, use StructDefs)
	StructDefs            []*StructDefinition // struct definitions with dependency info
	TypeDefs              []string            // typedef declarations for external types
	Includes              []string            // C header includes from cimport
	CImportLibraries      []string            // Libraries from cimport for pkg-config
	CImportObjects        []string            // Objects from cimport for linker input
	ExternalRootSymbols   []string            // User-declared external Gecko function definitions
	CurrentFunc           string
	CurrentFuncReturnType *tokens.TypeRef   // Return type of current function for validation
	LocalVars             map[string]string // variable name -> C type
	ChildContexts         map[string]*CScopeInformation
	TypeState             *ast.TypeState         // Flow-sensitive type state for this scope
	DeferStack            []string               // Deferred C code expressions to emit at scope exit
	ClosureCaptures       *ClosureCaptureContext // Active closure capture context for lambda compilation
	InUnsafe              bool                   // True inside @unsafe functions or @unsafe { ... } blocks

}

// UnsafeHandlerInstance records one active unsafe-handler value in scope:
// VarName is its C variable, TypeName is the handler type (used to look up
// which intrinsics it covers via the global UnsafeHandlerCoverage map).
type UnsafeHandlerInstance struct {
	VarName  string
	TypeName string
	Index    int
}

// ClosureCaptureContext tracks captured variables during lambda compilation
type ClosureCaptureContext struct {
	StructName string                          // Name of the capture struct (e.g., "__cap_0")
	ParamName  string                          // Parameter name in lambda (e.g., "__cap")
	Fields     []*ClosureCaptureField          // Captured variable fields
	FieldMap   map[string]*ClosureCaptureField // Quick lookup by variable name
	StructDef  string                          // Generated C struct definition
	GlobalSlot string                          // Global variable name for the capture
	OuterScope *ast.Ast                        // The enclosing scope where variables are defined
}

// ClosureCaptureField represents a single captured variable
type ClosureCaptureField struct {
	VarName   string   // Original variable name
	FullName  string   // Full qualified name
	CType     string   // C type of the variable
	IsPointer bool     // Whether the variable is a pointer
	Scope     *ast.Ast // The scope where the variable is defined
}

// TreeshakeDynamicCallWarning tracks dynamic-call patterns that require treeshake fallback.
type TreeshakeDynamicCallWarning struct {
	File   string
	Line   int
	Column int
	Reason string
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
	Backend interfaces.BackendInterface
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

// CState holds all per-compilation mutable state for the C backend.
// Previously these were package-level globals that needed ResetState().
type CState struct {
	ScopeDataMap                        *CScopeData
	ProgramValues                       *CValuesMap
	MethodSignatures                    map[string]*MethodSignature
	TraitDefinitions                    map[string]*tokens.Trait
	TraitDefinitionOrigins              map[string]string
	EnumToCType                         map[string]string
	MethodReturnTypes                   map[string]*tokens.TypeRef
	Generics                            *GenericRegistry
	MonomorphContext                    *MonomorphContext
	Methods                             map[string]*ast.Method
	Backend                             interfaces.BackendInterface
	TypeState                           *ast.TypeState
	LastCImportLibraries                []string
	LastCImportObjects                  []string
	LastTreeshakeAutoDisabled           bool
	LastTreeshakeDisableWarnings        []TreeshakeDynamicCallWarning
	currentTreeshakeDynamicCallWarnings []TreeshakeDynamicCallWarning
}

// NewCState creates a fresh compilation state.
func NewCState() *CState {
	return &CState{
		ScopeDataMap:           &CScopeData{},
		ProgramValues:          &CValuesMap{},
		MethodSignatures:       make(map[string]*MethodSignature),
		TraitDefinitions:       make(map[string]*tokens.Trait),
		TraitDefinitionOrigins: make(map[string]string),
		EnumToCType:            make(map[string]string),
		MethodReturnTypes:      make(map[string]*tokens.TypeRef),
		Generics:               NewGenericRegistry(),
		Methods:                make(map[string]*ast.Method),
	}
}

// CScopeDataMap holds all scope data
var CScopeDataMap = &CScopeData{}

// LastCImportLibraries holds libraries from the most recent compilation
// This allows the build command to access pkg-config libraries from cimport statements
var LastCImportLibraries []string

// LastCImportObjects holds object files from the most recent compilation.
var LastCImportObjects []string

// LastTreeshakeAutoDisabled reports whether treeshake was auto-disabled in the most recent compile.
var LastTreeshakeAutoDisabled bool

// LastTreeshakeDisableWarnings are warnings emitted when treeshake was auto-disabled.
var LastTreeshakeDisableWarnings []TreeshakeDynamicCallWarning

var currentTreeshakeDynamicCallWarnings []TreeshakeDynamicCallWarning

// CProgramValues holds all value info
var CProgramValues = &CValuesMap{}

// TraitDefinitions stores trait token definitions for default implementations
// Maps trait name (e.g., "Iterator") to its token definition
var TraitDefinitions = make(map[string]*tokens.Trait)

// TraitDefinitionOrigins stores the defining package for trait declarations.
// Maps trait name (e.g., "Iterator") to origin package (e.g., "std.core").
var TraitDefinitionOrigins = make(map[string]string)

// EnumToCType maps enum names to their mangled C type names
// Separate from GeckoToCType to avoid loadPrimitives overwriting enum ASTs
var EnumToCType = make(map[string]string)

// PendingCaptureInit stores capture initialization code generated during lambda compilation
// Key: lambda position (line:col), Value: capture initialization C code
var PendingCaptureInit = make(map[string]string)

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
	info.CImportLibraries = make([]string, 0)
	info.CImportObjects = make([]string, 0)
	info.ExternalRootSymbols = make([]string, 0)
	info.LocalVars = make(map[string]string)
	info.ChildContexts = make(map[string]*CScopeInformation)
	info.TypeState = ast.NewTypeState()
}

// ResetTreeshakeAnalysis clears per-compilation treeshake analysis state.
func ResetTreeshakeAnalysis() {
	LastTreeshakeAutoDisabled = false
	LastTreeshakeDisableWarnings = nil
	currentTreeshakeDynamicCallWarnings = nil
}

// RecordTreeshakeDynamicCall records a dynamic-call pattern that is unsafe for v1 static reachability.
func RecordTreeshakeDynamicCall(pos lexer.Position, reason string) {
	warn := TreeshakeDynamicCallWarning{
		File:   pos.Filename,
		Line:   pos.Line,
		Column: pos.Column,
		Reason: reason,
	}

	key := fmt.Sprintf("%s:%d:%d:%s", warn.File, warn.Line, warn.Column, warn.Reason)
	for _, existing := range currentTreeshakeDynamicCallWarnings {
		existingKey := fmt.Sprintf("%s:%d:%d:%s", existing.File, existing.Line, existing.Column, existing.Reason)
		if existingKey == key {
			return
		}
	}
	currentTreeshakeDynamicCallWarnings = append(currentTreeshakeDynamicCallWarnings, warn)
}

// GetTreeshakeDynamicCallWarnings returns a copy of detected dynamic-call warnings.
func GetTreeshakeDynamicCallWarnings() []TreeshakeDynamicCallWarning {
	out := make([]TreeshakeDynamicCallWarning, len(currentTreeshakeDynamicCallWarnings))
	copy(out, currentTreeshakeDynamicCallWarnings)
	return out
}
