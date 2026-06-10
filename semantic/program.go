// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md

package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

type DiagnosticSeverity string

const (
	SeverityError   DiagnosticSeverity = "error"
	SeverityWarning DiagnosticSeverity = "warning"
)

type DiagnosticKind string

const (
	DiagnosticInferenceAmbiguity DiagnosticKind = "inference_ambiguity"
	DiagnosticConstraintFailure  DiagnosticKind = "constraint_failure"
	DiagnosticTypeMismatch       DiagnosticKind = "type_mismatch"
)

type Diagnostic struct {
	Severity DiagnosticSeverity
	Kind     DiagnosticKind
	Title    string
	Message  string
	Help     string
	Pos      lexer.Position
}

type SymbolKind string

const (
	SymbolVariable  SymbolKind = "variable"
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolTypeParam SymbolKind = "type_param"
	SymbolClass     SymbolKind = "class"
)

type Symbol struct {
	ID       int64
	Kind     SymbolKind
	Name     string
	FullName string
	Type     *tokens.TypeRef
	Pos      lexer.Position
}

type TypeVar struct {
	ID      int64
	Name    string
	Traits  []string
	OwnerID int64
}

type FunctionSignature struct {
	SymbolID    int64
	Name        string
	FullName    string
	Module      string
	OwnerType   string
	ReturnType  *tokens.TypeRef
	Params      []*tokens.Value
	TypeParams  []*tokens.TypeParam
	Variadic    bool
	ExternalRef string
}

type ClassInfo struct {
	Name       string
	TypeParams []*tokens.TypeParam
	Fields     map[string]*tokens.TypeRef
}

type CallResolution struct {
	CalleeID         int64
	ReturnType       *tokens.TypeRef
	InferredTypeArgs map[string]*tokens.TypeRef
	UsedExplicitArgs bool
}

type FlowFacts struct {
	NonNullBySymbolID map[int64]bool
	NonNullByName     map[string]bool
}

type IfFacts struct {
	Then  *FlowFacts
	Else  *FlowFacts
	After *FlowFacts
}

type Program struct {
	File *tokens.File

	Symbols  map[int64]*Symbol
	TypeVars map[int64]*TypeVar

	expressionTypes map[*tokens.Expression]*tokens.TypeRef
	literalTypes    map[*tokens.Literal]*tokens.TypeRef
	funcCalls       map[*tokens.FuncCall]*CallResolution
	methodCalls     map[*tokens.MethodCall]*CallResolution
	ifFacts         map[*tokens.If]*IfFacts
	entryFacts      map[*tokens.Entry]*FlowFacts
	diagnostics     []Diagnostic

	functionsByName map[string][]*FunctionSignature
	moduleFunctions map[string]map[string][]*FunctionSignature
	staticMethods   map[string]map[string][]*FunctionSignature
	classes         map[string]*ClassInfo
	traitParents    map[string]string
	typeTraits      map[string]map[string]bool
	globalsByName   map[string]*tokens.TypeRef
	globalSymbolIDs map[string]int64

	nextID int64
}

func NewProgram(file *tokens.File) *Program {
	return &Program{
		File:            file,
		Symbols:         make(map[int64]*Symbol),
		TypeVars:        make(map[int64]*TypeVar),
		expressionTypes: make(map[*tokens.Expression]*tokens.TypeRef),
		literalTypes:    make(map[*tokens.Literal]*tokens.TypeRef),
		funcCalls:       make(map[*tokens.FuncCall]*CallResolution),
		methodCalls:     make(map[*tokens.MethodCall]*CallResolution),
		ifFacts:         make(map[*tokens.If]*IfFacts),
		entryFacts:      make(map[*tokens.Entry]*FlowFacts),
		functionsByName: make(map[string][]*FunctionSignature),
		moduleFunctions: make(map[string]map[string][]*FunctionSignature),
		staticMethods:   make(map[string]map[string][]*FunctionSignature),
		classes:         make(map[string]*ClassInfo),
		traitParents:    make(map[string]string),
		typeTraits:      make(map[string]map[string]bool),
		globalsByName:   make(map[string]*tokens.TypeRef),
		globalSymbolIDs: make(map[string]int64),
	}
}

func (p *Program) nextSymbolID() int64 {
	p.nextID++
	return p.nextID
}

func (p *Program) addSymbol(kind SymbolKind, name string, fullName string, typ *tokens.TypeRef, pos lexer.Position) int64 {
	id := p.nextSymbolID()
	p.Symbols[id] = &Symbol{
		ID:       id,
		Kind:     kind,
		Name:     name,
		FullName: fullName,
		Type:     CloneTypeRef(typ),
		Pos:      pos,
	}
	return id
}

func (p *Program) addTypeVar(name string, traits []string, ownerID int64) int64 {
	id := p.nextSymbolID()
	p.TypeVars[id] = &TypeVar{ID: id, Name: name, Traits: append([]string{}, traits...), OwnerID: ownerID}
	return id
}

func (p *Program) addDiagnostic(diag Diagnostic) {
	p.diagnostics = append(p.diagnostics, diag)
}

func (p *Program) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, len(p.diagnostics))
	copy(out, p.diagnostics)
	return out
}

func (p *Program) TypeOfExpression(expr *tokens.Expression) *tokens.TypeRef {
	if expr == nil {
		return nil
	}
	if t, ok := p.expressionTypes[expr]; ok {
		return CloneTypeRef(t)
	}
	return nil
}

func (p *Program) TypeOfLiteral(lit *tokens.Literal) *tokens.TypeRef {
	if lit == nil {
		return nil
	}
	if t, ok := p.literalTypes[lit]; ok {
		return CloneTypeRef(t)
	}
	return nil
}

func (p *Program) FuncCallResolution(call *tokens.FuncCall) *CallResolution {
	if call == nil {
		return nil
	}
	res, ok := p.funcCalls[call]
	if !ok || res == nil {
		return nil
	}
	return cloneCallResolution(res)
}

func (p *Program) MethodCallResolution(call *tokens.MethodCall) *CallResolution {
	if call == nil {
		return nil
	}
	res, ok := p.methodCalls[call]
	if !ok || res == nil {
		return nil
	}
	return cloneCallResolution(res)
}

func (p *Program) IfFlowFacts(ifStmt *tokens.If) *IfFacts {
	if ifStmt == nil {
		return nil
	}
	facts, ok := p.ifFacts[ifStmt]
	if !ok || facts == nil {
		return nil
	}
	return &IfFacts{
		Then:  CloneFlowFacts(facts.Then),
		Else:  CloneFlowFacts(facts.Else),
		After: CloneFlowFacts(facts.After),
	}
}

func (p *Program) EntryFlowFacts(entry *tokens.Entry) *FlowFacts {
	if entry == nil {
		return nil
	}
	facts, ok := p.entryFacts[entry]
	if !ok || facts == nil {
		return nil
	}
	return CloneFlowFacts(facts)
}

func cloneCallResolution(in *CallResolution) *CallResolution {
	if in == nil {
		return nil
	}
	out := &CallResolution{
		CalleeID:         in.CalleeID,
		ReturnType:       CloneTypeRef(in.ReturnType),
		InferredTypeArgs: make(map[string]*tokens.TypeRef, len(in.InferredTypeArgs)),
		UsedExplicitArgs: in.UsedExplicitArgs,
	}
	for k, v := range in.InferredTypeArgs {
		out.InferredTypeArgs[k] = CloneTypeRef(v)
	}
	return out
}

func NewFlowFacts() *FlowFacts {
	return &FlowFacts{
		NonNullBySymbolID: make(map[int64]bool),
		NonNullByName:     make(map[string]bool),
	}
}

func CloneFlowFacts(in *FlowFacts) *FlowFacts {
	if in == nil {
		return nil
	}
	out := NewFlowFacts()
	for id, val := range in.NonNullBySymbolID {
		if val {
			out.NonNullBySymbolID[id] = true
		}
	}
	for name, val := range in.NonNullByName {
		if val {
			out.NonNullByName[name] = true
		}
	}
	return out
}

func FlowFactsFromNonNull(nonNull map[int64]bool, nameForID map[int64]string) *FlowFacts {
	facts := NewFlowFacts()
	for id, ok := range nonNull {
		if !ok {
			continue
		}
		facts.NonNullBySymbolID[id] = true
		if name, exists := nameForID[id]; exists && name != "" {
			facts.NonNullByName[name] = true
		}
	}
	return facts
}

func MergeFlowFactsIntersection(a, b *FlowFacts) *FlowFacts {
	if a == nil && b == nil {
		return NewFlowFacts()
	}
	if a == nil {
		return CloneFlowFacts(b)
	}
	if b == nil {
		return CloneFlowFacts(a)
	}
	out := NewFlowFacts()
	for id := range a.NonNullBySymbolID {
		if b.NonNullBySymbolID[id] {
			out.NonNullBySymbolID[id] = true
		}
	}
	for name := range a.NonNullByName {
		if b.NonNullByName[name] {
			out.NonNullByName[name] = true
		}
	}
	return out
}

func CloneTypeRef(t *tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	out := &tokens.TypeRef{
		Module:   t.Module,
		Type:     t.Type,
		Trait:    t.Trait,
		Const:    t.Const,
		Volatile: t.Volatile,
		Pointer:  t.Pointer,
		NonNull:  t.NonNull,
	}
	if t.Array != nil {
		out.Array = CloneTypeRef(t.Array)
	}
	if t.Size != nil {
		out.Size = &tokens.SizeDef{Size: t.Size.Size, Type: CloneTypeRef(t.Size.Type)}
	}
	if t.FuncType != nil {
		params := make([]*tokens.TypeRef, len(t.FuncType.ParamTypes))
		for i, p := range t.FuncType.ParamTypes {
			params[i] = CloneTypeRef(p)
		}
		out.FuncType = &tokens.FuncType{
			ParamTypes: params,
			ReturnType: CloneTypeRef(t.FuncType.ReturnType),
			Throws:     CloneTypeRef(t.FuncType.Throws),
		}
	}
	if len(t.TypeArgs) > 0 {
		out.TypeArgs = make([]*tokens.TypeRef, len(t.TypeArgs))
		for i, arg := range t.TypeArgs {
			out.TypeArgs[i] = CloneTypeRef(arg)
		}
	}
	return out
}

func SubstituteTypeParams(t *tokens.TypeRef, subst map[string]*tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	if subst != nil {
		if concrete, ok := subst[t.Type]; ok && t.Array == nil && t.Size == nil && t.FuncType == nil {
			resolved := CloneTypeRef(concrete)
			if t.Pointer {
				resolved.Pointer = true
			}
			if t.NonNull {
				resolved.NonNull = true
			}
			if t.Const {
				resolved.Const = true
			}
			if t.Volatile {
				resolved.Volatile = true
			}
			return resolved
		}
	}

	out := CloneTypeRef(t)
	if out.Array != nil {
		out.Array = SubstituteTypeParams(out.Array, subst)
	}
	if out.Size != nil {
		out.Size.Type = SubstituteTypeParams(out.Size.Type, subst)
	}
	if out.FuncType != nil {
		for i, param := range out.FuncType.ParamTypes {
			out.FuncType.ParamTypes[i] = SubstituteTypeParams(param, subst)
		}
		out.FuncType.ReturnType = SubstituteTypeParams(out.FuncType.ReturnType, subst)
		out.FuncType.Throws = SubstituteTypeParams(out.FuncType.Throws, subst)
	}
	if len(out.TypeArgs) > 0 {
		for i, arg := range out.TypeArgs {
			out.TypeArgs[i] = SubstituteTypeParams(arg, subst)
		}
	}
	return out
}

func TypeRefString(t *tokens.TypeRef) string {
	if t == nil {
		return "unknown"
	}
	if t.Array != nil {
		return "[]" + TypeRefString(t.Array)
	}
	if t.Size != nil {
		return fmt.Sprintf("[%s]%s", t.Size.Size, TypeRefString(t.Size.Type))
	}
	if t.FuncType != nil {
		parts := make([]string, 0, len(t.FuncType.ParamTypes))
		for _, p := range t.FuncType.ParamTypes {
			parts = append(parts, TypeRefString(p))
		}
		ret := "void"
		if t.FuncType.ReturnType != nil {
			ret = TypeRefString(t.FuncType.ReturnType)
		}
		return "func(" + strings.Join(parts, ", ") + "): " + ret
	}

	name := t.Type
	if t.Module != "" {
		name = t.Module + "." + name
	}
	if len(t.TypeArgs) > 0 {
		args := make([]string, 0, len(t.TypeArgs))
		for _, arg := range t.TypeArgs {
			args = append(args, TypeRefString(arg))
		}
		name += "<" + strings.Join(args, ", ") + ">"
	}
	if t.Volatile {
		name += " volatile"
	}
	if t.Pointer {
		name += "*"
	}
	if t.NonNull {
		name += "!"
	}
	return name
}

func IsNumericType(t *tokens.TypeRef) bool {
	if t == nil {
		return false
	}
	switch t.Type {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float", "float32", "float64":
		return true
	default:
		return false
	}
}

func NormalizeTypeName(name string) string {
	if name == "" {
		return ""
	}
	aliases := map[string]string{
		"float": "float32",
	}
	if normalized, ok := aliases[name]; ok {
		return normalized
	}
	return name
}

func TypesEqual(a, b *tokens.TypeRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if NormalizeTypeName(a.Type) != NormalizeTypeName(b.Type) {
		return false
	}
	if a.Module != b.Module || a.Pointer != b.Pointer || a.NonNull != b.NonNull || a.Const != b.Const || a.Volatile != b.Volatile {
		return false
	}
	if (a.Array == nil) != (b.Array == nil) {
		return false
	}
	if a.Array != nil && !TypesEqual(a.Array, b.Array) {
		return false
	}
	if (a.Size == nil) != (b.Size == nil) {
		return false
	}
	if a.Size != nil {
		if a.Size.Size != b.Size.Size || !TypesEqual(a.Size.Type, b.Size.Type) {
			return false
		}
	}
	if len(a.TypeArgs) != len(b.TypeArgs) {
		return false
	}
	for i := range a.TypeArgs {
		if !TypesEqual(a.TypeArgs[i], b.TypeArgs[i]) {
			return false
		}
	}
	return true
}

func TypesCompatible(expected, actual *tokens.TypeRef) bool {
	if expected == nil || actual == nil {
		return true
	}
	if TypesEqual(expected, actual) {
		return true
	}

	if IsNumericType(expected) && IsNumericType(actual) {
		return true
	}

	// Nullable pointer cannot flow into non-null pointer.
	if expected.Pointer && expected.NonNull && (!actual.Pointer || !actual.NonNull) {
		return false
	}
	if expected.Pointer && actual.Pointer {
		if NormalizeTypeName(expected.Type) == "void" || NormalizeTypeName(actual.Type) == "void" {
			return true
		}
		if NormalizeTypeName(expected.Type) == NormalizeTypeName(actual.Type) {
			return true
		}
	}

	// C interop commonly uses both `string` and `string*!` at call boundaries.
	if NormalizeTypeName(expected.Type) == "string" && NormalizeTypeName(actual.Type) == "string" {
		return true
	}

	if NormalizeTypeName(expected.Type) == NormalizeTypeName(actual.Type) && expected.Pointer == actual.Pointer {
		return true
	}

	return false
}

func collectUnresolvedTypeParams(typeParams []*tokens.TypeParam, subst map[string]*tokens.TypeRef) []string {
	var unresolved []string
	for _, tp := range typeParams {
		if tp == nil {
			continue
		}
		if _, ok := subst[tp.Name]; !ok {
			unresolved = append(unresolved, tp.Name)
		}
	}
	sort.Strings(unresolved)
	return unresolved
}
