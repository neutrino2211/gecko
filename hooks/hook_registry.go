package hooks

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

// HookType represents the different kinds of compiler hooks
type HookType string

const (
	// Lifecycle hooks
	HookDrop HookType = "drop_hook"

	// Arithmetic operator hooks
	HookAdd HookType = "add_hook"
	HookSub HookType = "sub_hook"
	HookMul HookType = "mul_hook"
	HookDiv HookType = "div_hook"
	HookNeg HookType = "neg_hook"

	// Comparison operator hooks
	HookEq HookType = "eq_hook"
	HookNe HookType = "ne_hook"
	HookLt HookType = "lt_hook"
	HookGt HookType = "gt_hook"
	HookLe HookType = "le_hook"
	HookGe HookType = "ge_hook"

	// Bitwise operator hooks
	HookBitAnd HookType = "bitand_hook"
	HookBitOr  HookType = "bitor_hook"
	HookBitXor HookType = "bitxor_hook"
	HookShl    HookType = "shl_hook"
	HookShr    HookType = "shr_hook"

	// Indexing hooks
	HookIndex    HookType = "index_hook"
	HookIndexMut HookType = "index_mut_hook"

	// Iterator hooks
	HookIterator     HookType = "iterator_hook"
	HookIntoIterator HookType = "into_iterator_hook"

	// Error handling hooks
	HookTry HookType = "try_hook"
	HookOr  HookType = "or_hook"
)

// HookSignature describes the expected signature for a hook
type HookSignature struct {
	MethodCount int      // Number of methods (e.g., iterator has 2: next, has_next)
	HasSelf     bool     // Whether methods should have self parameter
	ParamCount  int      // Additional parameters (not counting self)
	ReturnType  string   // Expected return type pattern ("void", "bool", "Self", "T", "any")
}

// Known hook signatures
var hookSignatures = map[HookType]HookSignature{
	// Lifecycle
	HookDrop: {MethodCount: 1, HasSelf: true, ParamCount: 0, ReturnType: "void"},

	// Arithmetic (binary operators return T, unary returns Self)
	HookAdd: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookSub: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookMul: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookDiv: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookNeg: {MethodCount: 1, HasSelf: true, ParamCount: 0, ReturnType: "Self"},

	// Comparison (return bool)
	HookEq: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},
	HookNe: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},
	HookLt: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},
	HookGt: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},
	HookLe: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},
	HookGe: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "bool"},

	// Bitwise
	HookBitAnd: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookBitOr:  {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookBitXor: {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookShl:    {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookShr:    {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},

	// Indexing
	HookIndex:    {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
	HookIndexMut: {MethodCount: 1, HasSelf: true, ParamCount: 2, ReturnType: "void"},

	// Iterator (special: has 2 methods)
	HookIterator:     {MethodCount: 2, HasSelf: true, ParamCount: 0, ReturnType: "any"},
	HookIntoIterator: {MethodCount: 1, HasSelf: true, ParamCount: 0, ReturnType: "any"},

	// Error handling
	HookTry: {MethodCount: 1, HasSelf: true, ParamCount: 0, ReturnType: "T"},
	HookOr:  {MethodCount: 1, HasSelf: true, ParamCount: 1, ReturnType: "T"},
}

// RegisteredHook represents a trait registered as a hook
type RegisteredHook struct {
	TraitName   string   // Name of the trait
	HookType    HookType // Type of hook
	Methods     []string // Method names to call (from hook attribute)
	ModulePath  string   // Module where this hook is defined
	Pos         lexer.Position
}

// HookRegistry tracks all registered hooks per module
type HookRegistry struct {
	// hooks maps module path -> hook type -> registered hook
	hooks map[string]map[HookType]*RegisteredHook
}

// Global hook registry
var globalHookRegistry *HookRegistry

// GetHookRegistry returns the global hook registry
func GetHookRegistry() *HookRegistry {
	if globalHookRegistry == nil {
		globalHookRegistry = &HookRegistry{
			hooks: make(map[string]map[HookType]*RegisteredHook),
		}
	}
	return globalHookRegistry
}

// ResetHookRegistry clears the registry
func ResetHookRegistry() {
	globalHookRegistry = nil
}

// Register adds a hook to the registry for a specific module
func (r *HookRegistry) Register(modulePath string, hook *RegisteredHook, errorScope *errors.ErrorScope) bool {
	if r.hooks[modulePath] == nil {
		r.hooks[modulePath] = make(map[HookType]*RegisteredHook)
	}

	// Check for duplicate hook registration
	if existing, ok := r.hooks[modulePath][hook.HookType]; ok {
		errorScope.NewCompileTimeError(
			"Duplicate Hook",
			"Hook '"+string(hook.HookType)+"' is already registered by trait '"+existing.TraitName+"' in this module",
			hook.Pos,
		)
		return false
	}

	r.hooks[modulePath][hook.HookType] = hook
	return true
}

// GetHook returns the registered hook for a type in the given module (or imported modules)
func (r *HookRegistry) GetHook(modulePath string, hookType HookType) *RegisteredHook {
	if moduleHooks, ok := r.hooks[modulePath]; ok {
		if hook, ok := moduleHooks[hookType]; ok {
			return hook
		}
	}
	return nil
}

// GetHookFromAnyModule searches all registered modules for a hook of the given type.
// Use this when the trait may be defined in an imported module.
func (r *HookRegistry) GetHookFromAnyModule(hookType HookType) *RegisteredHook {
	for _, moduleHooks := range r.hooks {
		if hook, ok := moduleHooks[hookType]; ok {
			return hook
		}
	}
	return nil
}

// GetAllHooksForModule returns all hooks registered in a module
func (r *HookRegistry) GetAllHooksForModule(modulePath string) map[HookType]*RegisteredHook {
	if hooks, ok := r.hooks[modulePath]; ok {
		return hooks
	}
	return nil
}

// ParseHookAttribute checks if an attribute is a hook and returns its type
func ParseHookAttribute(attr *tokens.Attribute) (HookType, bool) {
	if attr == nil {
		return "", false
	}

	hookType := HookType(attr.Name)
	if _, known := hookSignatures[hookType]; known {
		return hookType, true
	}
	return "", false
}

// ValidateTraitForHook verifies a trait matches the expected signature for a hook
func ValidateTraitForHook(trait *tokens.Trait, hookType HookType, methods []string, errorScope *errors.ErrorScope) bool {
	sig, ok := hookSignatures[hookType]
	if !ok {
		return false
	}

	// Check method count
	if len(methods) != sig.MethodCount {
		errorScope.NewCompileTimeError(
			"Hook Signature Error",
			"Hook '"+string(hookType)+"' expects "+string(rune('0'+sig.MethodCount))+" method(s), but "+string(rune('0'+len(methods)))+" provided",
			trait.Pos,
		)
		return false
	}

	// Verify each method exists in the trait
	traitMethods := make(map[string]*tokens.Method)
	for _, f := range trait.Fields {
		m := f.ToMethodToken()
		traitMethods[m.Name] = m
	}

	for _, methodName := range methods {
		method, exists := traitMethods[methodName]
		if !exists {
			errorScope.NewCompileTimeError(
				"Hook Signature Error",
				"Method '"+methodName+"' specified in hook but not found in trait '"+trait.Name+"'",
				trait.Pos,
			)
			return false
		}

		// Validate method signature
		if !validateMethodSignature(method, sig, hookType, errorScope) {
			return false
		}
	}

	return true
}

// validateMethodSignature checks if a method matches the expected hook signature
func validateMethodSignature(method *tokens.Method, sig HookSignature, hookType HookType, errorScope *errors.ErrorScope) bool {
	// Check self parameter
	hasSelf := false
	paramCount := 0
	for _, arg := range method.Arguments {
		if arg.Name == "self" {
			hasSelf = true
		} else {
			paramCount++
		}
	}

	if sig.HasSelf && !hasSelf {
		errorScope.NewCompileTimeError(
			"Hook Signature Error",
			"Method '"+method.Name+"' for hook '"+string(hookType)+"' must have 'self' parameter",
			method.Pos,
		)
		return false
	}

	if paramCount != sig.ParamCount {
		errorScope.NewCompileTimeError(
			"Hook Signature Error",
			"Method '"+method.Name+"' for hook '"+string(hookType)+"' expects "+string(rune('0'+sig.ParamCount))+" parameter(s) (excluding self), got "+string(rune('0'+paramCount)),
			method.Pos,
		)
		return false
	}

	// Check return type (basic validation)
	if sig.ReturnType != "any" {
		returnType := "void"
		if method.Type != nil {
			returnType = method.Type.Type
		}

		valid := false
		switch sig.ReturnType {
		case "void":
			valid = returnType == "void" || returnType == ""
		case "bool":
			valid = returnType == "bool"
		case "Self", "T":
			// These are generic - any non-void type is acceptable
			valid = returnType != "void" && returnType != ""
		}

		if !valid {
			errorScope.NewCompileTimeError(
				"Hook Signature Error",
				"Method '"+method.Name+"' for hook '"+string(hookType)+"' has wrong return type. Expected '"+sig.ReturnType+"', got '"+returnType+"'",
				method.Pos,
			)
			return false
		}
	}

	return true
}

// ProcessTraitHooks checks a trait for hook attributes and registers them
func ProcessTraitHooks(trait *tokens.Trait, modulePath string, errorScope *errors.ErrorScope) {
	registry := GetHookRegistry()

	for _, attr := range trait.Attributes {
		hookType, isHook := ParseHookAttribute(attr)
		if !isHook {
			continue
		}

		methods := attr.GetHookMethods()
		if len(methods) == 0 {
			errorScope.NewCompileTimeError(
				"Hook Error",
				"Hook '"+string(hookType)+"' requires method reference(s), e.g., @"+string(hookType)+"(.methodName)",
				attr.Pos,
			)
			continue
		}

		// Validate the trait signature
		if !ValidateTraitForHook(trait, hookType, methods, errorScope) {
			continue
		}

		// Register the hook
		hook := &RegisteredHook{
			TraitName:  trait.Name,
			HookType:   hookType,
			Methods:    methods,
			ModulePath: modulePath,
			Pos:        trait.Pos,
		}

		registry.Register(modulePath, hook, errorScope)
	}
}

// OperatorToHook maps operators to their hook types
var OperatorToHook = map[string]HookType{
	"+":  HookAdd,
	"-":  HookSub,
	"*":  HookMul,
	"/":  HookDiv,
	"==": HookEq,
	"!=": HookNe,
	"<":  HookLt,
	">":  HookGt,
	"<=": HookLe,
	">=": HookGe,
	"&":  HookBitAnd,
	"|":  HookBitOr,
	"^":  HookBitXor,
	"<<": HookShl,
	">>": HookShr,
}

// FormatHookInfo returns a human-readable description of a hook
func (h *RegisteredHook) FormatHookInfo() string {
	var sb strings.Builder
	sb.WriteString("Hook: " + string(h.HookType) + "\n")
	sb.WriteString("  Trait: " + h.TraitName + "\n")
	sb.WriteString("  Methods: " + strings.Join(h.Methods, ", ") + "\n")
	sb.WriteString("  Module: " + h.ModulePath + "\n")
	return sb.String()
}
