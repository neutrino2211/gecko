// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"

	"github.com/neutrino2211/gecko/tokens"
)

type analyzer struct {
	program *Program

	visitedFiles map[string]bool
	nameForID    map[int64]string

	currentFunction *FunctionSignature
	currentReturn   *tokens.TypeRef
	loopDepth       int

	unsafeBlockCounter int
	setupNames         map[string]bool
}

type resolutionAttempt struct {
	signature *FunctionSignature
	returnTyp *tokens.TypeRef
	subst     map[string]*tokens.TypeRef
}

func Analyze(file *tokens.File) *Program {
	prog := NewProgram(file)
	a := &analyzer{
		program:      prog,
		visitedFiles: make(map[string]bool),
		nameForID:    make(map[int64]string),
		setupNames:   make(map[string]bool),
	}

	a.indexFileRecursive(file)
	a.analyzeFileRecursive(file)
	return prog
}

func (a *analyzer) analyzeMethod(module, ownerType string, method *tokens.Method) {
	if method == nil {
		return
	}
	sig := a.findExactSignature(module, ownerType, method.Name)
	if sig == nil {
		return
	}

	// Set Self type for resolving Self in method signatures
	oldSelfType := CurrentSelfType
	if ownerType != "" {
		CurrentSelfType = ownerType
	}
	defer func() {
		CurrentSelfType = oldSelfType
	}()

	env := newFlowEnv()
	for globalName, typ := range a.program.globalsByName {
		sid := a.program.globalSymbolIDs[globalName]
		env.bind(globalName, typ, sid)
		a.nameForID[sid] = globalName
	}

	for _, arg := range method.Arguments {
		if arg == nil {
			continue
		}
		argType := CloneTypeRef(arg.Type)
		if argType == nil && arg.Name == "self" && ownerType != "" {
			argType = &tokens.TypeRef{Type: ownerType}
		}
		full := sig.FullName + "::" + arg.Name
		sid := a.program.addSymbol(SymbolVariable, arg.Name, full, argType, arg.Pos)
		a.nameForID[sid] = arg.Name
		env.bind(arg.Name, argType, sid)
	}

	prevFn := a.currentFunction
	prevRet := a.currentReturn
	a.currentFunction = sig
	a.currentReturn = CloneTypeRef(sig.ReturnType)
	_ = a.analyzeEntries(method.Value, env)
	a.currentFunction = prevFn
	a.currentReturn = prevRet
}

func (a *analyzer) findExactSignature(module, ownerType, name string) *FunctionSignature {
	cands := a.findFunctionCandidates(module, ownerType, name)
	if len(cands) == 0 {
		return nil
	}
	return cands[0]
}

func (a *analyzer) analyzeEntries(entries []*tokens.Entry, in *flowEnv) *flowEnv {
	env := in.clone()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		a.annotateEntryFacts(entry, env)
		if env.exited {
			continue
		}

		switch {
		case entry.Field != nil:
			env = a.analyzeFieldDecl(entry.Field, env)
		case entry.Destructuring != nil:
			env = a.analyzeDestructuringDecl(entry.Destructuring, env)
		case entry.Assignment != nil:
			env = a.analyzeAssignment(entry.Assignment, env)
		case entry.Return != nil:
			a.inferExpression(entry.Return, env, a.currentReturn)
			env.exited = true
		case entry.VoidReturn != nil:
			env.exited = true
		case entry.If != nil:
			env = a.analyzeIf(entry.If, env)
		case entry.Loop != nil:
			env = a.analyzeLoop(entry.Loop, env)
		case entry.FuncCall != nil:
			a.inferFuncCall(entry.FuncCall, env, nil)
		case entry.MethodCall != nil:
			a.inferMethodCallStatement(entry.MethodCall, env)
		case entry.Intrinsic != nil:
			for _, arg := range entry.Intrinsic.Args {
				a.inferExpression(arg, env, nil)
			}
		case entry.UnsafeBlock != nil:
			a.analyzeUnsafeBlock(entry.UnsafeBlock, env)
		}
	}
	return env
}

func (a *analyzer) analyzeDestructuringDecl(d *tokens.DestructuringDeclaration, env *flowEnv) *flowEnv {
	out := env.clone()
	valueType := a.inferExpression(d.Value, out, nil)
	if valueType == nil {
		return out
	}

	class, ok := a.program.classes[valueType.Type]
	if !ok {
		return out
	}

	for _, binding := range d.Bindings {
		if binding == nil {
			continue
		}
		fieldType, ok := class.Fields[binding.Field]
		if !ok || fieldType == nil {
			continue
		}
		target := binding.Target()
		full := target
		if a.currentFunction != nil {
			full = a.currentFunction.FullName + "::" + target + fmt.Sprintf("@%d:%d", binding.Pos.Line, binding.Pos.Column)
		}
		sid := a.program.addSymbol(SymbolVariable, target, full, CloneTypeRef(fieldType), binding.Pos)
		a.nameForID[sid] = target
		out.bind(target, CloneTypeRef(fieldType), sid)
	}
	return out
}

func (a *analyzer) analyzeFieldDecl(field *tokens.Field, env *flowEnv) *flowEnv {
	out := env.clone()

	// `@unsafe with ... { }` used as the initializer of a `let`/`const` binds the
	// block's Result<T, E> directly into the variable (e.g. `let r = @unsafe ...`).
	if ub := field.Value.GetUnsafeBlock(); ub != nil {
		tokens.UnsafeBlockBindNames[ub] = field.Name
		resType := a.analyzeUnsafeBlock(ub, out)
		if resType == nil {
			resType = &tokens.TypeRef{Type: "int"}
		}
		full := field.Name
		if a.currentFunction != nil {
			full = a.currentFunction.FullName + "::" + field.Name + fmt.Sprintf("@%d:%d", field.Pos.Line, field.Pos.Column)
		}
		sid := a.program.addSymbol(SymbolVariable, field.Name, full, resType, field.Pos)
		a.nameForID[sid] = field.Name
		out.bind(field.Name, resType, sid)
		return out
	}

	declType := CloneTypeRef(field.Type)
	valueType := a.inferExpression(field.Value, out, declType)
	finalType := declType
	if finalType == nil {
		finalType = CloneTypeRef(valueType)
	}
	if finalType == nil {
		a.program.addDiagnostic(Diagnostic{
			Severity: SeverityError,
			Kind:     DiagnosticTypeMismatch,
			Title:    "Type Inference Error",
			Message:  "Unable to infer variable type; please provide an explicit type annotation",
			Pos:      field.Pos,
		})
		finalType = &tokens.TypeRef{Type: "int"}
	}

	if declType != nil && valueType != nil && !TypesCompatible(declType, valueType) {
		a.program.addDiagnostic(Diagnostic{
			Severity: SeverityError,
			Kind:     DiagnosticTypeMismatch,
			Title:    "Type Mismatch",
			Message:  fmt.Sprintf("Cannot initialize '%s' of type '%s' with '%s'", field.Name, TypeRefString(declType), TypeRefString(valueType)),
			Pos:      field.Pos,
		})
	}

	full := field.Name
	if a.currentFunction != nil {
		full = a.currentFunction.FullName + "::" + field.Name + fmt.Sprintf("@%d:%d", field.Pos.Line, field.Pos.Column)
	}
	sid := a.program.addSymbol(SymbolVariable, field.Name, full, finalType, field.Pos)
	a.nameForID[sid] = field.Name
	out.bind(field.Name, finalType, sid)
	if finalType.Pointer && !finalType.NonNull {
		out.invalidateNonNull(sid)
	}
	return out
}

func (a *analyzer) analyzeAssignment(assign *tokens.Assignment, env *flowEnv) *flowEnv {
	out := env.clone()

	// Field/index assignments are validated by backend structural typing today.
	// Keep semantic pass conservative here to avoid false positives like `p.x = 1`.
	if assign.Field != "" || assign.Index != nil {
		a.inferExpression(assign.Value, out, nil)
		return out
	}

	// `@unsafe with ... { }` used as the RHS of an assignment (e.g. `r = @unsafe ...`)
	// reuses an already-declared Result variable: the block binds into it, reusing
	// the existing variable's error-enum type so the new value matches its type.
	if ub := assign.Value.GetUnsafeBlock(); ub != nil {
		if target := out.lookup(assign.Name); target != nil && target.Type != nil &&
			target.Type.Type == "Result" && len(target.Type.TypeArgs) == 2 {
			tokens.UnsafeBlockErrorNames[ub] = target.Type.TypeArgs[1].Type
		}
		tokens.UnsafeBlockBindNames[ub] = assign.Name
		a.analyzeUnsafeBlock(ub, out)
		return out
	}

	target := out.lookup(assign.Name)
	var expected *tokens.TypeRef
	if target != nil {
		expected = target.Type
	}
	actual := a.inferExpression(assign.Value, out, expected)

	if target != nil && expected != nil && actual != nil && !TypesCompatible(expected, actual) {
		a.program.addDiagnostic(Diagnostic{
			Severity: SeverityError,
			Kind:     DiagnosticTypeMismatch,
			Title:    "Type Mismatch",
			Message:  fmt.Sprintf("Cannot assign '%s' to '%s' of type '%s'", TypeRefString(actual), assign.Name, TypeRefString(expected)),
			Pos:      assign.Pos,
		})
	}

	if target != nil {
		if actual != nil && actual.Pointer && actual.NonNull {
			out.setNonNull(target.SymbolID)
		} else {
			out.invalidateNonNull(target.SymbolID)
		}
	}
	return out
}
