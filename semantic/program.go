// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

// CurrentSelfType tracks the current class type for resolving `Self` in method signatures.
var CurrentSelfType string

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
	borrowHooks     map[string]map[string]string
	typeTraits      map[string]map[string]bool
	globalsByName   map[string]*tokens.TypeRef
	globalSymbolIDs map[string]int64

	// signatureBySymbolID links a function/method symbol to its resolved
	// signature. It lets the language server read the exact (generic-substituted,
	// trait/default-impl resolved) signature the compiler uses.
	signatureBySymbolID map[int64]*FunctionSignature

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
		borrowHooks:     make(map[string]map[string]string),
		typeTraits:      make(map[string]map[string]bool),
		globalsByName:   make(map[string]*tokens.TypeRef),
		globalSymbolIDs: make(map[string]int64),

		signatureBySymbolID: make(map[int64]*FunctionSignature),
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

// Class returns the recorded class info for a type name, or nil.
func (p *Program) Class(name string) *ClassInfo {
	return p.classes[name]
}

// MethodsOfType returns all methods (instance and static) registered for a type.
func (p *Program) MethodsOfType(typeName string) []*FunctionSignature {
	out := make([]*FunctionSignature, 0)
	for _, sigs := range p.staticMethods[typeName] {
		out = append(out, sigs...)
	}
	return out
}

// ModuleFunctions returns every function declared in a module.
func (p *Program) ModuleFunctions(module string) []*FunctionSignature {
	out := make([]*FunctionSignature, 0)
	for _, sigs := range p.moduleFunctions[module] {
		out = append(out, sigs...)
	}
	return out
}

// FunctionsNamed returns every function/method with the given simple name.
func (p *Program) FunctionsNamed(name string) []*FunctionSignature {
	return append([]*FunctionSignature{}, p.functionsByName[name]...)
}

// GlobalType returns the resolved type of a top-level variable, or nil.
func (p *Program) GlobalType(name string) *tokens.TypeRef {
	return CloneTypeRef(p.globalsByName[name])
}

// SymbolByID returns the symbol with the given ID, or nil.
func (p *Program) SymbolByID(id int64) *Symbol {
	return p.Symbols[id]
}

// SignatureForSymbol returns the resolved signature for a function/method
// symbol, or nil. This is the compiler's authoritative view of the signature
// (after generic substitution and trait/default-impl resolution).
func (p *Program) SignatureForSymbol(id int64) *FunctionSignature {
	return p.signatureBySymbolID[id]
}
