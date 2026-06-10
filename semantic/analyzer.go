// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md

package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

type analyzer struct {
	program *Program

	visitedFiles map[string]bool
	nameForID    map[int64]string

	currentFunction *FunctionSignature
	currentReturn   *tokens.TypeRef
	loopDepth       int
}

type boundVar struct {
	Name     string
	Type     *tokens.TypeRef
	SymbolID int64
}

type flowEnv struct {
	vars    map[string]*boundVar
	nonNull map[int64]bool
	exited  bool
}

type resolutionAttempt struct {
	signature *FunctionSignature
	returnTyp *tokens.TypeRef
	subst     map[string]*tokens.TypeRef
}

type conditionFacts struct {
	trueNonNull  map[int64]bool
	falseNonNull map[int64]bool
}

func Analyze(file *tokens.File) *Program {
	prog := NewProgram(file)
	a := &analyzer{
		program:      prog,
		visitedFiles: make(map[string]bool),
		nameForID:    make(map[int64]string),
	}

	a.indexFileRecursive(file)
	a.analyzeFileRecursive(file)
	return prog
}

func (a *analyzer) fileKey(file *tokens.File) string {
	if file == nil {
		return ""
	}
	if file.Path != "" {
		return file.Path
	}
	if file.Name != "" {
		return file.Name
	}
	return fmt.Sprintf("%p", file)
}

func moduleNameForFile(file *tokens.File) string {
	if file == nil {
		return ""
	}
	if file.Name != "" {
		return file.Name
	}
	return file.PackageName
}

func (a *analyzer) indexFileRecursive(file *tokens.File) {
	if file == nil {
		return
	}
	key := a.fileKey(file)
	if key != "" && a.visitedFiles[key] {
		return
	}
	if key != "" {
		a.visitedFiles[key] = true
	}

	module := moduleNameForFile(file)
	a.indexFileEntries(file, module)

	for _, imported := range file.Imports {
		a.indexFileRecursive(imported)
	}
}

func (a *analyzer) indexFileEntries(file *tokens.File, module string) {
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}

		if entry.Trait != nil {
			if entry.Trait.Parent != "" {
				a.program.traitParents[entry.Trait.Name] = entry.Trait.Parent
			}
		}

		if entry.Field != nil {
			full := entry.Field.Name
			if module != "" {
				full = module + "::" + entry.Field.Name
			}
			id := a.program.addSymbol(SymbolVariable, entry.Field.Name, full, entry.Field.Type, entry.Field.Pos)
			a.program.globalsByName[entry.Field.Name] = CloneTypeRef(entry.Field.Type)
			a.program.globalSymbolIDs[entry.Field.Name] = id
			a.nameForID[id] = entry.Field.Name
		}

		if entry.Method != nil {
			a.registerFunctionSignature(module, "", entry.Method.Name, entry.Method.Type, entry.Method.Arguments, entry.Method.TypeParams, entry.Method.IsVariadic(), entry.Method.Pos)
		}

		if entry.Declaration != nil && entry.Declaration.Method != nil {
			m := entry.Declaration.Method
			a.registerFunctionSignature(module, "", m.Name, m.Type, m.Arguments, m.TypeParams, m.IsVariadic(), m.Pos)
		}

		if entry.Foreign != nil {
			for _, member := range entry.Foreign.Members {
				if member == nil || member.Method == nil {
					continue
				}
				m := member.Method
				a.registerFunctionSignature(entry.Foreign.Module, "", m.Name, m.Type, m.Arguments, nil, m.IsVariadic(), m.Pos)
			}
		}

		if entry.Class != nil {
			cls := entry.Class
			full := cls.Name
			if module != "" {
				full = module + "::" + cls.Name
			}
			id := a.program.addSymbol(SymbolClass, cls.Name, full, &tokens.TypeRef{Type: cls.Name}, cls.Pos)
			a.nameForID[id] = cls.Name

			classInfo := &ClassInfo{
				Name:       cls.Name,
				TypeParams: cls.TypeParams,
				Fields:     make(map[string]*tokens.TypeRef),
			}
			for _, field := range cls.Fields {
				if field == nil {
					continue
				}
				if field.Field != nil {
					classInfo.Fields[field.Field.Name] = CloneTypeRef(field.Field.Type)
				}
				if field.Method != nil {
					a.registerFunctionSignature(module, cls.Name, field.Method.Name, field.Method.Type, field.Method.Arguments, field.Method.TypeParams, field.Method.IsVariadic(), field.Method.Pos)
				}
			}
			a.program.classes[cls.Name] = classInfo
		}

		if entry.Implementation != nil {
			impl := entry.Implementation
			if impl.GetFor() != "" && impl.GetName() != "" {
				a.addTraitImpl(impl.GetFor(), impl.GetName())
			}
			if impl.GetFor() != "" {
				for _, field := range impl.GetFields() {
					if field == nil {
						continue
					}
					m := field.ToMethodToken()
					a.registerFunctionSignature(module, impl.GetFor(), m.Name, m.Type, m.Arguments, m.TypeParams, m.IsVariadic(), m.Pos)
				}
			}
		}
	}
}

func (a *analyzer) analyzeFileRecursive(file *tokens.File) {
	if file == nil {
		return
	}

	visited := make(map[string]bool)
	var walk func(*tokens.File)
	walk = func(f *tokens.File) {
		if f == nil {
			return
		}
		key := a.fileKey(f) + "#analyze"
		if visited[key] {
			return
		}
		visited[key] = true

		module := moduleNameForFile(f)
		a.analyzeFileEntries(f, module)
		for _, imported := range f.Imports {
			walk(imported)
		}
	}

	walk(file)
}

func (a *analyzer) analyzeFileEntries(file *tokens.File, module string) {
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Method != nil {
			a.analyzeMethod(module, "", entry.Method)
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field != nil && field.Method != nil {
					a.analyzeMethod(module, entry.Class.Name, field.Method)
				}
			}
		}
		if entry.Implementation != nil && entry.Implementation.GetFor() != "" {
			for _, field := range entry.Implementation.GetFields() {
				if field == nil {
					continue
				}
				m := field.ToMethodToken()
				a.analyzeMethod(module, entry.Implementation.GetFor(), m)
			}
		}
	}
}

func (a *analyzer) registerFunctionSignature(module, ownerType, name string, ret *tokens.TypeRef, params []*tokens.Value, typeParams []*tokens.TypeParam, variadic bool, pos lexer.Position) {
	_ = pos
	full := name
	if ownerType != "" {
		if module != "" {
			full = module + "::" + ownerType + "::" + name
		} else {
			full = ownerType + "::" + name
		}
	} else if module != "" {
		full = module + "::" + name
	}

	symbolKind := SymbolFunction
	if ownerType != "" {
		symbolKind = SymbolMethod
	}
	symID := a.program.addSymbol(symbolKind, name, full, ret, pos)

	sig := &FunctionSignature{
		SymbolID:   symID,
		Name:       name,
		FullName:   full,
		Module:     module,
		OwnerType:  ownerType,
		ReturnType: CloneTypeRef(ret),
		Params:     cloneValues(params),
		TypeParams: cloneTypeParams(typeParams),
		Variadic:   variadic,
	}

	a.program.functionsByName[name] = append(a.program.functionsByName[name], sig)
	if module != "" {
		if _, ok := a.program.moduleFunctions[module]; !ok {
			a.program.moduleFunctions[module] = make(map[string][]*FunctionSignature)
		}
		a.program.moduleFunctions[module][name] = append(a.program.moduleFunctions[module][name], sig)
	}
	if ownerType != "" {
		if _, ok := a.program.staticMethods[ownerType]; !ok {
			a.program.staticMethods[ownerType] = make(map[string][]*FunctionSignature)
		}
		a.program.staticMethods[ownerType][name] = append(a.program.staticMethods[ownerType][name], sig)
	}

	for _, tp := range typeParams {
		if tp == nil {
			continue
		}
		id := a.program.addTypeVar(tp.Name, tp.AllTraits(), symID)
		a.nameForID[id] = tp.Name
	}
}

func cloneValues(in []*tokens.Value) []*tokens.Value {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.Value, 0, len(in))
	for _, v := range in {
		if v == nil {
			continue
		}
		out = append(out, &tokens.Value{
			Variadic: v.Variadic,
			Name:     v.Name,
			Out:      v.Out,
			Type:     CloneTypeRef(v.Type),
			Default:  v.Default,
		})
	}
	return out
}

func cloneTypeParams(in []*tokens.TypeParam) []*tokens.TypeParam {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.TypeParam, 0, len(in))
	for _, tp := range in {
		if tp == nil {
			continue
		}
		cloned := &tokens.TypeParam{Name: tp.Name, Trait: tp.Trait}
		if len(tp.Traits) > 0 {
			cloned.Traits = append([]string{}, tp.Traits...)
		}
		out = append(out, cloned)
	}
	return out
}

func (a *analyzer) addTraitImpl(typeName string, traitName string) {
	if typeName == "" || traitName == "" {
		return
	}
	if _, ok := a.program.typeTraits[typeName]; !ok {
		a.program.typeTraits[typeName] = make(map[string]bool)
	}
	a.program.typeTraits[typeName][traitName] = true
}

func newFlowEnv() *flowEnv {
	return &flowEnv{
		vars:    make(map[string]*boundVar),
		nonNull: make(map[int64]bool),
	}
}

func (e *flowEnv) clone() *flowEnv {
	out := newFlowEnv()
	for name, v := range e.vars {
		out.vars[name] = &boundVar{Name: v.Name, Type: CloneTypeRef(v.Type), SymbolID: v.SymbolID}
	}
	for id, ok := range e.nonNull {
		if ok {
			out.nonNull[id] = true
		}
	}
	out.exited = e.exited
	return out
}

func (e *flowEnv) bind(name string, typ *tokens.TypeRef, symbolID int64) {
	e.vars[name] = &boundVar{Name: name, Type: CloneTypeRef(typ), SymbolID: symbolID}
	if typ != nil && typ.Pointer && typ.NonNull {
		e.nonNull[symbolID] = true
	}
}

func (e *flowEnv) lookup(name string) *boundVar {
	if v, ok := e.vars[name]; ok {
		return v
	}
	return nil
}

func (e *flowEnv) invalidateNonNull(symbolID int64) {
	delete(e.nonNull, symbolID)
}

func (e *flowEnv) setNonNull(symbolID int64) {
	e.nonNull[symbolID] = true
}

func (e *flowEnv) applyNonNull(facts map[int64]bool) {
	for id, ok := range facts {
		if ok {
			e.nonNull[id] = true
		}
	}
}

func (a *analyzer) annotateEntryFacts(entry *tokens.Entry, env *flowEnv) {
	if entry == nil || env == nil {
		return
	}
	a.program.entryFacts[entry] = FlowFactsFromNonNull(env.nonNull, a.nameForID)
}

func (a *analyzer) analyzeMethod(module, ownerType string, method *tokens.Method) {
	if method == nil {
		return
	}
	sig := a.findExactSignature(module, ownerType, method.Name)
	if sig == nil {
		return
	}

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
		}
	}
	return env
}

func (a *analyzer) analyzeFieldDecl(field *tokens.Field, env *flowEnv) *flowEnv {
	out := env.clone()
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

func (a *analyzer) analyzeIf(ifStmt *tokens.If, env *flowEnv) *flowEnv {
	condType := a.inferExpression(ifStmt.Expression, env, &tokens.TypeRef{Type: "bool"})
	if condType != nil {
		// keep for graph consumers
		a.program.expressionTypes[ifStmt.Expression] = CloneTypeRef(condType)
	}

	facts := a.extractConditionFacts(ifStmt.Expression, env)
	thenStart := env.clone()
	thenStart.applyNonNull(facts.trueNonNull)
	thenOut := a.analyzeEntries(ifStmt.Value, thenStart)

	elseStart := env.clone()
	elseStart.applyNonNull(facts.falseNonNull)
	elseOut := elseStart
	if ifStmt.ElseIf != nil {
		elseOut = a.analyzeElseIf(ifStmt.ElseIf, elseStart)
	} else if ifStmt.Else != nil {
		elseOut = a.analyzeEntries(ifStmt.Else.Value, elseStart)
	}

	out := mergeIfFlows(env, thenOut, elseOut)
	a.program.ifFacts[ifStmt] = &IfFacts{
		Then:  FlowFactsFromNonNull(thenStart.nonNull, a.nameForID),
		Else:  FlowFactsFromNonNull(elseStart.nonNull, a.nameForID),
		After: FlowFactsFromNonNull(out.nonNull, a.nameForID),
	}
	return out
}

func (a *analyzer) analyzeElseIf(ei *tokens.ElseIf, env *flowEnv) *flowEnv {
	condType := a.inferExpression(ei.Expression, env, &tokens.TypeRef{Type: "bool"})
	if condType != nil {
		a.program.expressionTypes[ei.Expression] = CloneTypeRef(condType)
	}

	facts := a.extractConditionFacts(ei.Expression, env)
	thenStart := env.clone()
	thenStart.applyNonNull(facts.trueNonNull)
	thenOut := a.analyzeEntries(ei.Value, thenStart)

	elseStart := env.clone()
	elseStart.applyNonNull(facts.falseNonNull)
	elseOut := elseStart
	if ei.ElseIf != nil {
		elseOut = a.analyzeElseIf(ei.ElseIf, elseStart)
	} else if ei.Else != nil {
		elseOut = a.analyzeEntries(ei.Else.Value, elseStart)
	}

	return mergeIfFlows(env, thenOut, elseOut)
}

func mergeIfFlows(base, thenOut, elseOut *flowEnv) *flowEnv {
	if thenOut.exited && elseOut.exited {
		out := base.clone()
		out.exited = true
		return out
	}
	if thenOut.exited {
		return sanitizeBranchToBase(base, elseOut)
	}
	if elseOut.exited {
		return sanitizeBranchToBase(base, thenOut)
	}

	left := sanitizeBranchToBase(base, thenOut)
	right := sanitizeBranchToBase(base, elseOut)
	out := base.clone()
	out.exited = false
	out.nonNull = make(map[int64]bool)
	for id := range left.nonNull {
		if right.nonNull[id] {
			out.nonNull[id] = true
		}
	}
	for name, baseVar := range out.vars {
		lv := left.vars[name]
		rv := right.vars[name]
		if lv == nil || rv == nil {
			out.vars[name] = baseVar
			continue
		}
		if TypesCompatible(lv.Type, rv.Type) {
			out.vars[name].Type = CloneTypeRef(lv.Type)
		} else {
			out.vars[name].Type = CloneTypeRef(baseVar.Type)
		}
	}
	return out
}

func sanitizeBranchToBase(base, branch *flowEnv) *flowEnv {
	out := base.clone()
	out.exited = branch.exited
	for name := range out.vars {
		if b, ok := branch.vars[name]; ok {
			out.vars[name] = &boundVar{Name: b.Name, Type: CloneTypeRef(b.Type), SymbolID: b.SymbolID}
		}
	}
	out.nonNull = make(map[int64]bool)
	for id, ok := range branch.nonNull {
		if ok {
			out.nonNull[id] = true
		}
	}
	return out
}

func (a *analyzer) analyzeLoop(loop *tokens.Loop, env *flowEnv) *flowEnv {
	out := env.clone()
	prevDepth := a.loopDepth
	a.loopDepth++
	defer func() {
		a.loopDepth = prevDepth
	}()

	if loop.ForExpression != nil {
		a.inferExpression(loop.ForExpression, out, &tokens.TypeRef{Type: "bool"})
	}
	if loop.WhileExpr != nil {
		a.inferExpression(loop.WhileExpr, out, &tokens.TypeRef{Type: "bool"})
	}
	if loop.ForOf != nil {
		a.inferExpression(loop.ForOf.SourceArray, out, nil)
		elem := &tokens.TypeRef{Type: "int32"}
		sourceType := a.inferExpression(loop.ForOf.SourceArray, out, nil)
		if sourceType != nil {
			if sourceType.Array != nil {
				elem = sourceType.Array
			} else if sourceType.Size != nil && sourceType.Size.Type != nil {
				elem = sourceType.Size.Type
			}
		}
		if loop.ForOf.Variable != nil {
			loopVar := CloneTypeRef(loop.ForOf.Variable.Type)
			if loopVar == nil {
				loopVar = CloneTypeRef(elem)
			}
			sid := a.program.addSymbol(SymbolVariable, loop.ForOf.Variable.Name, loop.ForOf.Variable.Name, loopVar, loop.ForOf.Variable.Pos)
			a.nameForID[sid] = loop.ForOf.Variable.Name
			bodyEnv := out.clone()
			bodyEnv.bind(loop.ForOf.Variable.Name, loopVar, sid)
			_ = a.analyzeEntries(loop.Value, bodyEnv)
		}
		return out
	}
	if loop.ForIn != nil {
		a.inferExpression(loop.ForIn.SourceArray, out, nil)
		if loop.ForIn.Variable != nil {
			loopVar := CloneTypeRef(loop.ForIn.Variable.Type)
			if loopVar == nil {
				loopVar = &tokens.TypeRef{Type: "int32"}
			}
			sid := a.program.addSymbol(SymbolVariable, loop.ForIn.Variable.Name, loop.ForIn.Variable.Name, loopVar, loop.ForIn.Variable.Pos)
			a.nameForID[sid] = loop.ForIn.Variable.Name
			bodyEnv := out.clone()
			bodyEnv.bind(loop.ForIn.Variable.Name, loopVar, sid)
			_ = a.analyzeEntries(loop.Value, bodyEnv)
		}
		return out
	}

	_ = a.analyzeEntries(loop.Value, out.clone())
	return out
}

func (a *analyzer) inferMethodCallStatement(call *tokens.MethodCall, env *flowEnv) {
	if call == nil {
		return
	}
	lit := &tokens.Literal{Symbol: call.Base, Chain: call.Chain}
	expr := &tokens.Expression{OrExpr: &tokens.OrExpression{LogicalOr: &tokens.LogicalOr{LogicalAnd: &tokens.LogicalAnd{Equality: &tokens.Equality{Comparison: &tokens.Comparison{Addition: &tokens.Addition{Multiplication: &tokens.Multiplication{Unary: &tokens.Unary{Primary: &tokens.Primary{Literal: lit}}}}}}}}}}
	a.inferExpression(expr, env, nil)
}

func (a *analyzer) inferExpression(expr *tokens.Expression, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if expr == nil || expr.OrExpr == nil {
		return nil
	}
	resolved := a.inferOrExpression(expr.OrExpr, env, expected)
	if resolved != nil {
		a.program.expressionTypes[expr] = CloneTypeRef(resolved)
	}
	return resolved
}

func (a *analyzer) inferOrExpression(orExpr *tokens.OrExpression, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if orExpr == nil {
		return nil
	}
	left := a.inferLogicalOr(orExpr.LogicalOr, env, expected)
	if orExpr.Or == nil {
		return left
	}
	right := a.inferOrExpression(orExpr.Or, env, expected)
	if right != nil {
		return right
	}
	return left
}

func (a *analyzer) inferLogicalOr(lo *tokens.LogicalOr, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if lo == nil {
		return nil
	}
	left := a.inferLogicalAnd(lo.LogicalAnd, env, expected)
	if lo.Next != nil {
		a.inferLogicalOr(lo.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferLogicalAnd(la *tokens.LogicalAnd, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if la == nil {
		return nil
	}
	left := a.inferEquality(la.Equality, env, expected)
	if la.Next != nil {
		a.inferLogicalAnd(la.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferEquality(eq *tokens.Equality, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if eq == nil {
		return nil
	}
	left := a.inferComparison(eq.Comparison, env, expected)
	if eq.Next != nil {
		a.inferEquality(eq.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferComparison(c *tokens.Comparison, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if c == nil {
		return nil
	}
	left := a.inferAddition(c.Addition, env, expected)
	if c.Next != nil {
		a.inferComparison(c.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferAddition(add *tokens.Addition, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if add == nil {
		return nil
	}
	left := a.inferMultiplication(add.Multiplication, env, expected)
	if add.Next == nil {
		return left
	}
	right := a.inferAddition(add.Next, env, expected)
	if IsNumericType(left) && IsNumericType(right) {
		if strings.HasPrefix(left.Type, "float") || strings.HasPrefix(right.Type, "float") {
			return &tokens.TypeRef{Type: "float64"}
		}
		if left.Type == "uint" || right.Type == "uint" {
			return &tokens.TypeRef{Type: "uint"}
		}
		return &tokens.TypeRef{Type: "int32"}
	}
	if left != nil {
		return left
	}
	return right
}

func (a *analyzer) inferMultiplication(mul *tokens.Multiplication, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if mul == nil {
		return nil
	}
	left := a.inferUnary(mul.Unary, env, expected)
	if mul.Next == nil {
		return left
	}
	right := a.inferMultiplication(mul.Next, env, expected)
	if IsNumericType(left) && IsNumericType(right) {
		if strings.HasPrefix(left.Type, "float") || strings.HasPrefix(right.Type, "float") {
			return &tokens.TypeRef{Type: "float64"}
		}
		if left.Type == "uint" || right.Type == "uint" {
			return &tokens.TypeRef{Type: "uint"}
		}
		return &tokens.TypeRef{Type: "int32"}
	}
	if left != nil {
		return left
	}
	return right
}

func (a *analyzer) inferUnary(un *tokens.Unary, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if un == nil {
		return nil
	}
	if un.Unary != nil {
		inner := a.inferUnary(un.Unary, env, expected)
		if un.Op == "!" {
			return &tokens.TypeRef{Type: "bool"}
		}
		if un.Op == "try" {
			if inner != nil && len(inner.TypeArgs) > 0 {
				return CloneTypeRef(inner.TypeArgs[0])
			}
		}
		return inner
	}

	if un.Primary != nil {
		result := a.inferPrimary(un.Primary, env, expected)
		if un.Cast != nil && un.Cast.Type != nil {
			return CloneTypeRef(un.Cast.Type)
		}
		return result
	}
	return nil
}

func (a *analyzer) inferPrimary(p *tokens.Primary, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if p == nil {
		return nil
	}
	if p.SubExpression != nil {
		return a.inferExpression(p.SubExpression, env, expected)
	}
	return a.inferLiteral(p.Literal, env, expected)
}

func (a *analyzer) inferLiteral(l *tokens.Literal, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if l == nil {
		return nil
	}

	var inferred *tokens.TypeRef
	switch {
	case l.Number != "":
		if strings.Contains(l.Number, ".") {
			inferred = &tokens.TypeRef{Type: "float64"}
		} else {
			inferred = &tokens.TypeRef{Type: "int32"}
		}
	case l.Bool != "":
		inferred = &tokens.TypeRef{Type: "bool"}
	case l.HasStringLiteral():
		inferred = &tokens.TypeRef{Type: "string"}
	case l.Intrinsic != nil:
		inferred = a.inferIntrinsic(l.Intrinsic, env)
	case l.FuncCall != nil:
		inferred = a.inferFuncCall(l.FuncCall, env, expected)
	case (l.Symbol == "nil" || l.Symbol == "null") && len(l.Chain) == 0:
		inferred = &tokens.TypeRef{Type: "void", Pointer: true}
	case l.Symbol != "":
		inferred = a.inferSymbolLiteral(l, env)
	case l.StructType != "":
		inferred = &tokens.TypeRef{Type: l.StructType, TypeArgs: cloneTypeRefSlice(l.StructTypeArgs)}
	case len(l.Array) > 0:
		first := a.inferLiteral(l.Array[0], env, nil)
		if first != nil {
			inferred = &tokens.TypeRef{Array: CloneTypeRef(first)}
		}
	}

	if inferred != nil && l.IsPointer {
		inferred = &tokens.TypeRef{Type: inferred.Type, TypeArgs: cloneTypeRefSlice(inferred.TypeArgs), Pointer: true, NonNull: true}
	}
	if inferred != nil {
		a.program.literalTypes[l] = CloneTypeRef(inferred)
	}
	return inferred
}

func cloneTypeRefSlice(in []*tokens.TypeRef) []*tokens.TypeRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.TypeRef, len(in))
	for i := range in {
		out[i] = CloneTypeRef(in[i])
	}
	return out
}

func (a *analyzer) inferSymbolLiteral(l *tokens.Literal, env *flowEnv) *tokens.TypeRef {
	var current *tokens.TypeRef
	if v := env.lookup(l.Symbol); v != nil {
		current = CloneTypeRef(v.Type)
		if current != nil && current.Pointer && env.nonNull[v.SymbolID] {
			current.NonNull = true
		}
	} else if gt, ok := a.program.globalsByName[l.Symbol]; ok {
		current = CloneTypeRef(gt)
		if current != nil && current.Pointer {
			if sid, exists := a.program.globalSymbolIDs[l.Symbol]; exists && env.nonNull[sid] {
				current.NonNull = true
			}
		}
	}

	if current == nil {
		return nil
	}

	if l.ArrayIndex != nil {
		a.inferExpression(l.ArrayIndex, env, &tokens.TypeRef{Type: "int32"})
		if current.Array != nil {
			current = CloneTypeRef(current.Array)
		} else if current.Size != nil {
			current = CloneTypeRef(current.Size.Type)
		}
	}

	for _, chain := range l.Chain {
		if chain == nil {
			continue
		}
		if chain.IsMethodCall() {
			ret := a.inferMethodOnType(current, chain, env)
			if ret == nil {
				return nil
			}
			current = ret
			continue
		}
		classInfo := a.program.classes[current.Type]
		if classInfo == nil {
			return nil
		}
		fieldType := classInfo.Fields[chain.Name]
		if fieldType == nil {
			return nil
		}
		subst := classTypeSubst(current, classInfo.TypeParams)
		current = SubstituteTypeParams(fieldType, subst)
	}

	return current
}

func classTypeSubst(instanceType *tokens.TypeRef, params []*tokens.TypeParam) map[string]*tokens.TypeRef {
	if instanceType == nil || len(params) == 0 || len(instanceType.TypeArgs) == 0 {
		return nil
	}
	subst := make(map[string]*tokens.TypeRef)
	for i, p := range params {
		if i >= len(instanceType.TypeArgs) || p == nil {
			continue
		}
		subst[p.Name] = CloneTypeRef(instanceType.TypeArgs[i])
	}
	return subst
}

func (a *analyzer) inferMethodOnType(receiver *tokens.TypeRef, chain *tokens.ChainAccess, env *flowEnv) *tokens.TypeRef {
	if receiver == nil || chain == nil {
		return nil
	}
	candidates := a.program.staticMethods[receiver.Type][chain.Name]
	if len(candidates) == 0 {
		return nil
	}
	attempt := a.resolveCallCandidates(candidates, chain.TypeArgs, chain.Args, receiver, nil, env, chain.Pos)
	if attempt == nil {
		return nil
	}
	return CloneTypeRef(attempt.returnTyp)
}

func (a *analyzer) inferIntrinsic(intr *tokens.Intrinsic, env *flowEnv) *tokens.TypeRef {
	if intr == nil {
		return nil
	}
	for _, arg := range intr.Args {
		a.inferExpression(arg, env, nil)
	}
	switch intr.Name {
	case "size_of":
		return &tokens.TypeRef{Type: "uint64"}
	case "is_null", "is_not_null":
		return &tokens.TypeRef{Type: "bool"}
	default:
		return nil
	}
}

func (a *analyzer) inferFuncCall(call *tokens.FuncCall, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if call == nil {
		return nil
	}
	candidates := a.lookupCallCandidates(call)
	if len(candidates) == 0 {
		return nil
	}

	attempt := a.resolveCallCandidates(candidates, call.TypeArgs, call.Arguments, nil, expected, env, call.Pos)
	if attempt == nil {
		return nil
	}

	res := &CallResolution{
		CalleeID:         attempt.signature.SymbolID,
		ReturnType:       CloneTypeRef(attempt.returnTyp),
		InferredTypeArgs: make(map[string]*tokens.TypeRef),
		UsedExplicitArgs: len(call.TypeArgs) > 0,
	}
	for name, typ := range attempt.subst {
		res.InferredTypeArgs[name] = CloneTypeRef(typ)
	}
	a.program.funcCalls[call] = res
	return CloneTypeRef(attempt.returnTyp)
}

func (a *analyzer) lookupCallCandidates(call *tokens.FuncCall) []*FunctionSignature {
	if call == nil {
		return nil
	}
	if call.StaticType != "" {
		cands := a.program.staticMethods[call.StaticType][call.Function]
		if call.StaticModule == "" {
			return append([]*FunctionSignature{}, cands...)
		}
		filtered := make([]*FunctionSignature, 0, len(cands))
		for _, c := range cands {
			if c.Module == call.StaticModule {
				filtered = append(filtered, c)
			}
		}
		return filtered
	}
	if call.Module != "" {
		if modFns, ok := a.program.moduleFunctions[call.Module]; ok {
			return append([]*FunctionSignature{}, modFns[call.Function]...)
		}
		return nil
	}
	return append([]*FunctionSignature{}, a.program.functionsByName[call.Function]...)
}

func (a *analyzer) resolveCallCandidates(candidates []*FunctionSignature, explicitTypeArgs []*tokens.TypeRef, args []*tokens.Argument, receiver *tokens.TypeRef, expected *tokens.TypeRef, env *flowEnv, pos lexer.Position) *resolutionAttempt {
	_ = pos
	if len(candidates) == 0 {
		return nil
	}
	valid := make([]*resolutionAttempt, 0)
	for _, cand := range candidates {
		if cand == nil {
			continue
		}
		attempt := a.tryResolveCandidate(cand, explicitTypeArgs, args, receiver, expected, env)
		if attempt != nil {
			valid = append(valid, attempt)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	if len(valid) > 1 {
		hasGenericCandidate := false
		for _, attempt := range valid {
			if attempt != nil && attempt.signature != nil && len(attempt.signature.TypeParams) > 0 {
				hasGenericCandidate = true
				break
			}
		}
		if hasGenericCandidate {
			a.program.addDiagnostic(Diagnostic{
				Severity: SeverityError,
				Kind:     DiagnosticInferenceAmbiguity,
				Title:    "Type Inference Ambiguity",
				Message:  "Multiple callable overloads matched; provide explicit type arguments to disambiguate",
				Help:     "Use explicit `<...>` type arguments on the call.",
			})
			return nil
		}
		return valid[0]
	}
	return valid[0]
}

func (a *analyzer) tryResolveCandidate(sig *FunctionSignature, explicitTypeArgs []*tokens.TypeRef, args []*tokens.Argument, receiver *tokens.TypeRef, expected *tokens.TypeRef, env *flowEnv) *resolutionAttempt {
	subst := make(map[string]*tokens.TypeRef)
	typeParamsByName := make(map[string]*tokens.TypeParam)
	for _, tp := range sig.TypeParams {
		if tp == nil {
			continue
		}
		typeParamsByName[tp.Name] = tp
	}

	if len(explicitTypeArgs) > 0 {
		if len(explicitTypeArgs) != len(sig.TypeParams) {
			return nil
		}
		for i, tp := range sig.TypeParams {
			subst[tp.Name] = CloneTypeRef(explicitTypeArgs[i])
		}
	}

	paramStart := 0
	if receiver != nil && len(sig.Params) > 0 {
		first := sig.Params[0]
		if first != nil && first.Name == "self" {
			formal := CloneTypeRef(first.Type)
			if formal == nil && sig.OwnerType != "" {
				formal = &tokens.TypeRef{Type: sig.OwnerType}
			}
			if formal != nil {
				if err := unifyType(formal, receiver, subst, typeParamsByName); err != nil {
					if receiver.Pointer {
						alt := CloneTypeRef(receiver)
						alt.Pointer = false
						alt.NonNull = false
						if err2 := unifyType(formal, alt, subst, typeParamsByName); err2 != nil {
							return nil
						}
					} else {
						return nil
					}
				}
			}
			paramStart = 1
		}
	}

	params := sig.Params[paramStart:]
	fixedCount := 0
	for _, p := range params {
		if p != nil && p.Variadic {
			break
		}
		fixedCount++
	}

	if len(args) < fixedCount {
		return nil
	}
	if !sig.Variadic && len(args) > len(params) {
		return nil
	}

	for i, arg := range args {
		if arg == nil {
			continue
		}
		var param *tokens.Value
		if i < len(params) {
			param = params[i]
		} else if len(params) > 0 {
			param = params[len(params)-1]
		}

		var expectedArg *tokens.TypeRef
		if param != nil {
			expectedArg = SubstituteTypeParams(param.Type, subst)
		}
		actual := a.inferArgumentType(arg, env, expectedArg)
		if param != nil && param.Type != nil {
			if err := unifyType(param.Type, actual, subst, typeParamsByName); err != nil {
				return nil
			}
		}
		concreteExpected := expectedArg
		if param != nil && param.Type != nil {
			concreteExpected = SubstituteTypeParams(param.Type, subst)
		}
		if concreteExpected != nil && actual != nil && !isUnresolvedTypeParamRef(concreteExpected, typeParamsByName) && !TypesCompatible(concreteExpected, actual) {
			return nil
		}
	}

	if expected != nil && sig.ReturnType != nil {
		if err := unifyType(sig.ReturnType, expected, subst, typeParamsByName); err != nil {
			return nil
		}
	}

	unresolved := collectUnresolvedTypeParams(sig.TypeParams, subst)
	if len(unresolved) > 0 {
		help := "Use explicit `<...>` type arguments on this call."
		sort.Strings(unresolved)
		a.program.addDiagnostic(Diagnostic{
			Severity: SeverityError,
			Kind:     DiagnosticInferenceAmbiguity,
			Title:    "Type Inference Ambiguity",
			Message:  fmt.Sprintf("Could not infer type arguments for %s: %s", sig.Name, strings.Join(unresolved, ", ")),
			Help:     help,
		})
		return nil
	}

	for _, tp := range sig.TypeParams {
		if tp == nil {
			continue
		}
		resolved := subst[tp.Name]
		if resolved == nil {
			continue
		}
		for _, required := range tp.AllTraits() {
			if !a.typeImplementsTrait(resolved, required) {
				a.program.addDiagnostic(Diagnostic{
					Severity: SeverityError,
					Kind:     DiagnosticConstraintFailure,
					Title:    "Generic Constraint Error",
					Message:  fmt.Sprintf("Type '%s' does not satisfy constraint '%s' for type parameter '%s'", TypeRefString(resolved), required, tp.Name),
				})
				return nil
			}
		}
	}

	ret := SubstituteTypeParams(sig.ReturnType, subst)
	if expected != nil && ret != nil && !TypesCompatible(expected, ret) {
		return nil
	}
	return &resolutionAttempt{signature: sig, returnTyp: ret, subst: subst}
}

func (a *analyzer) inferArgumentType(arg *tokens.Argument, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if arg == nil {
		return nil
	}
	if arg.Value != nil {
		return a.inferExpression(arg.Value, env, expected)
	}
	if arg.SubCall != nil {
		return a.inferFuncCall(arg.SubCall, env, expected)
	}
	return nil
}

func unifyType(formal, actual *tokens.TypeRef, subst map[string]*tokens.TypeRef, typeParams map[string]*tokens.TypeParam) error {
	if formal == nil || actual == nil {
		return nil
	}

	if _, isTypeParam := typeParams[formal.Type]; isTypeParam && formal.Array == nil && formal.Size == nil && formal.FuncType == nil && len(formal.TypeArgs) == 0 {
		existing := subst[formal.Type]
		if existing == nil {
			subst[formal.Type] = CloneTypeRef(actual)
			return nil
		}
		if !TypesCompatible(existing, actual) || !TypesCompatible(actual, existing) {
			return fmt.Errorf("conflicting inference for type parameter '%s'", formal.Type)
		}
		return nil
	}

	if formal.Array != nil || actual.Array != nil {
		if formal.Array == nil || actual.Array == nil {
			return fmt.Errorf("array mismatch")
		}
		return unifyType(formal.Array, actual.Array, subst, typeParams)
	}

	if formal.Size != nil || actual.Size != nil {
		if formal.Size == nil || actual.Size == nil {
			return fmt.Errorf("sized array mismatch")
		}
		return unifyType(formal.Size.Type, actual.Size.Type, subst, typeParams)
	}

	if formal.FuncType != nil || actual.FuncType != nil {
		if formal.FuncType == nil || actual.FuncType == nil {
			return fmt.Errorf("function type mismatch")
		}
		if len(formal.FuncType.ParamTypes) != len(actual.FuncType.ParamTypes) {
			return fmt.Errorf("function arity mismatch")
		}
		for i := range formal.FuncType.ParamTypes {
			if err := unifyType(formal.FuncType.ParamTypes[i], actual.FuncType.ParamTypes[i], subst, typeParams); err != nil {
				return err
			}
		}
		return unifyType(formal.FuncType.ReturnType, actual.FuncType.ReturnType, subst, typeParams)
	}

	if formal.Pointer != actual.Pointer {
		return fmt.Errorf("pointer mismatch")
	}

	if NormalizeTypeName(formal.Type) != NormalizeTypeName(actual.Type) {
		if !(IsNumericType(formal) && IsNumericType(actual)) {
			return fmt.Errorf("type mismatch")
		}
	}

	if len(formal.TypeArgs) > 0 {
		if len(formal.TypeArgs) != len(actual.TypeArgs) {
			return fmt.Errorf("type arg arity mismatch")
		}
		for i := range formal.TypeArgs {
			if err := unifyType(formal.TypeArgs[i], actual.TypeArgs[i], subst, typeParams); err != nil {
				return err
			}
		}
	}

	return nil
}

func isUnresolvedTypeParamRef(t *tokens.TypeRef, typeParams map[string]*tokens.TypeParam) bool {
	if t == nil {
		return false
	}
	if _, ok := typeParams[t.Type]; ok && t.Array == nil && t.Size == nil && t.FuncType == nil && len(t.TypeArgs) == 0 {
		return true
	}
	return false
}

func (a *analyzer) typeImplementsTrait(t *tokens.TypeRef, required string) bool {
	if t == nil || required == "" {
		return false
	}
	typeName := t.Type
	if typeName == "" {
		return false
	}
	if a.program.typeTraits[typeName][required] {
		return true
	}
	for implemented := range a.program.typeTraits[typeName] {
		if a.traitExtends(implemented, required) {
			return true
		}
	}
	return false
}

func (a *analyzer) traitExtends(child string, parent string) bool {
	if child == parent {
		return true
	}
	seen := make(map[string]bool)
	cur := child
	for cur != "" {
		if seen[cur] {
			break
		}
		seen[cur] = true
		next := a.program.traitParents[cur]
		if next == parent {
			return true
		}
		cur = next
	}
	return false
}

func (a *analyzer) findFunctionCandidates(module, ownerType, name string) []*FunctionSignature {
	if ownerType != "" {
		return append([]*FunctionSignature{}, a.program.staticMethods[ownerType][name]...)
	}
	if module != "" {
		if mod, ok := a.program.moduleFunctions[module]; ok {
			return append([]*FunctionSignature{}, mod[name]...)
		}
	}
	return append([]*FunctionSignature{}, a.program.functionsByName[name]...)
}

func (a *analyzer) extractConditionFacts(expr *tokens.Expression, env *flowEnv) conditionFacts {
	if expr == nil || expr.GetLogicalOr() == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	return a.factsFromLogicalOr(expr.GetLogicalOr(), env)
}

func (a *analyzer) factsFromLogicalOr(lo *tokens.LogicalOr, env *flowEnv) conditionFacts {
	if lo == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	left := a.factsFromLogicalAnd(lo.LogicalAnd, env)
	if lo.Next == nil {
		return left
	}
	right := a.factsFromLogicalOr(lo.Next, env)
	return conditionFacts{
		trueNonNull:  intersectFactMaps(left.trueNonNull, right.trueNonNull),
		falseNonNull: unionFactMaps(left.falseNonNull, right.falseNonNull),
	}
}

func (a *analyzer) factsFromLogicalAnd(la *tokens.LogicalAnd, env *flowEnv) conditionFacts {
	if la == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	left := a.factsFromEquality(la.Equality, env)
	if la.Next == nil {
		return left
	}
	right := a.factsFromLogicalAnd(la.Next, env)
	return conditionFacts{
		trueNonNull:  unionFactMaps(left.trueNonNull, right.trueNonNull),
		falseNonNull: intersectFactMaps(left.falseNonNull, right.falseNonNull),
	}
}

func (a *analyzer) factsFromEquality(eq *tokens.Equality, env *flowEnv) conditionFacts {
	if eq == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	if eq.Next != nil && (eq.Op == "!=" || eq.Op == "==") {
		leftSymbol := extractSymbolFromComparison(eq.Comparison)
		rightSymbol := extractSymbolFromEquality(eq.Next)
		leftNil := isNilFromComparison(eq.Comparison)
		rightNil := isNilFromEquality(eq.Next)

		if leftSymbol != "" && rightNil {
			return a.factsFromNullComparison(leftSymbol, eq.Op == "!=", env)
		}
		if rightSymbol != "" && leftNil {
			return a.factsFromNullComparison(rightSymbol, eq.Op == "!=", env)
		}
	}

	if info := a.factsFromIntrinsicEquality(eq, env); info.trueNonNull != nil {
		return info
	}

	// Nested comparisons don't carry non-null facts by default.
	return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
}

func (a *analyzer) factsFromNullComparison(symbol string, isNotNull bool, env *flowEnv) conditionFacts {
	out := conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	if symbol == "" {
		return out
	}
	bound := env.lookup(symbol)
	if bound == nil {
		if sid, ok := a.program.globalSymbolIDs[symbol]; ok {
			bound = &boundVar{Name: symbol, SymbolID: sid}
		}
	}
	if bound == nil || bound.SymbolID == 0 {
		return out
	}
	if isNotNull {
		out.trueNonNull[bound.SymbolID] = true
	} else {
		out.falseNonNull[bound.SymbolID] = true
	}
	return out
}

func (a *analyzer) factsFromIntrinsicEquality(eq *tokens.Equality, env *flowEnv) conditionFacts {
	out := conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	primary := primaryFromEquality(eq)
	if primary == nil || primary.Literal == nil || primary.Literal.Intrinsic == nil {
		return conditionFacts{}
	}
	intr := primary.Literal.Intrinsic
	if intr.Name != "is_not_null" && intr.Name != "is_null" {
		return conditionFacts{}
	}
	if len(intr.Args) != 1 {
		return conditionFacts{}
	}
	symbol := extractSymbolFromExpression(intr.Args[0])
	if symbol == "" {
		return conditionFacts{}
	}
	bound := env.lookup(symbol)
	if bound == nil {
		if sid, ok := a.program.globalSymbolIDs[symbol]; ok {
			bound = &boundVar{Name: symbol, SymbolID: sid}
		}
	}
	if bound == nil || bound.SymbolID == 0 {
		return conditionFacts{}
	}
	if intr.Name == "is_not_null" {
		out.trueNonNull[bound.SymbolID] = true
	} else {
		out.falseNonNull[bound.SymbolID] = true
	}
	return out
}

func unionFactMaps(a, b map[int64]bool) map[int64]bool {
	out := make(map[int64]bool)
	for id, ok := range a {
		if ok {
			out[id] = true
		}
	}
	for id, ok := range b {
		if ok {
			out[id] = true
		}
	}
	return out
}

func intersectFactMaps(a, b map[int64]bool) map[int64]bool {
	if len(a) == 0 || len(b) == 0 {
		return map[int64]bool{}
	}
	out := make(map[int64]bool)
	for id, ok := range a {
		if ok && b[id] {
			out[id] = true
		}
	}
	return out
}

func primaryFromEquality(eq *tokens.Equality) *tokens.Primary {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return nil
	}
	cmp := eq.Comparison
	if cmp.Next != nil || cmp.Addition == nil {
		return nil
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return nil
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil {
		return nil
	}
	if mul.Unary.Primary == nil {
		return nil
	}
	return mul.Unary.Primary
}

func extractSymbolFromExpression(expr *tokens.Expression) string {
	if expr == nil || expr.GetLogicalOr() == nil {
		return ""
	}
	lo := expr.GetLogicalOr()
	if lo.Next != nil || lo.LogicalAnd == nil || lo.LogicalAnd.Next != nil || lo.LogicalAnd.Equality == nil {
		return ""
	}
	return extractSymbolFromEquality(lo.LogicalAnd.Equality)
}

func extractSymbolFromEquality(eq *tokens.Equality) string {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return ""
	}
	return extractSymbolFromComparison(eq.Comparison)
}

func extractSymbolFromComparison(cmp *tokens.Comparison) string {
	if cmp == nil || cmp.Next != nil || cmp.Addition == nil {
		return ""
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return ""
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil || mul.Unary.Primary == nil || mul.Unary.Primary.Literal == nil {
		return ""
	}
	lit := mul.Unary.Primary.Literal
	if lit.Symbol != "" && lit.SymbolModule == "" && len(lit.Chain) == 0 && lit.ArrayIndex == nil {
		return lit.Symbol
	}
	return ""
}

func isNilFromEquality(eq *tokens.Equality) bool {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return false
	}
	return isNilFromComparison(eq.Comparison)
}

func isNilFromComparison(cmp *tokens.Comparison) bool {
	if cmp == nil || cmp.Next != nil || cmp.Addition == nil {
		return false
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return false
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil || mul.Unary.Primary == nil || mul.Unary.Primary.Literal == nil {
		return false
	}
	lit := mul.Unary.Primary.Literal
	return (lit.Symbol == "nil" || lit.Symbol == "null") && lit.SymbolModule == "" && len(lit.Chain) == 0 && lit.ArrayIndex == nil
}
