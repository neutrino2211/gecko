// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"github.com/neutrino2211/gecko/tokens"
)

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

type conditionFacts struct {
	trueNonNull  map[int64]bool
	falseNonNull map[int64]bool
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
