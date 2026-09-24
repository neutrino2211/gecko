// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) inferFuncCall(call *tokens.FuncCall, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if call == nil {
		return nil
	}
	candidates := a.lookupCallCandidates(call)
	if len(candidates) == 0 && call.Module != "" {
		receiver := a.lookupReceiverType(call.Module, env)
		if receiver != nil {
			candidates = a.program.staticMethods[receiver.Type][call.Function]
			if len(candidates) > 0 {
				attempt := a.resolveCallCandidates(candidates, call.TypeArgs, call.Arguments, receiver, expected, env, call.Pos)
				if attempt != nil {
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
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	var ownerReceiver *tokens.TypeRef
	if call.StaticType != "" {
		ownerReceiver = &tokens.TypeRef{
			Type:     call.StaticType,
			TypeArgs: cloneTypeRefSlice(call.StaticTypeArgs),
		}
	}

	attempt := a.resolveCallCandidates(candidates, call.TypeArgs, call.Arguments, ownerReceiver, expected, env, call.Pos)
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

func (a *analyzer) lookupReceiverType(name string, env *flowEnv) *tokens.TypeRef {
	if name == "" {
		return nil
	}
	if env != nil {
		if v := env.lookup(name); v != nil {
			current := CloneTypeRef(v.Type)
			if current != nil && current.Pointer && env.nonNull[v.SymbolID] {
				current.NonNull = true
			}
			return current
		}
	}
	if gt, ok := a.program.globalsByName[name]; ok {
		current := CloneTypeRef(gt)
		if current != nil && current.Pointer {
			if sid, exists := a.program.globalSymbolIDs[name]; exists && env != nil && env.nonNull[sid] {
				current.NonNull = true
			}
		}
		return current
	}
	return nil
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

	// Set Self type for resolving Self in method signatures
	oldSelfType := CurrentSelfType
	if sig.OwnerType != "" {
		CurrentSelfType = sig.OwnerType
	}
	defer func() {
		CurrentSelfType = oldSelfType
	}()

	ownerTypeParams := a.ownerTypeParams(sig.OwnerType)
	for _, tp := range ownerTypeParams {
		if tp == nil {
			continue
		}
		if _, exists := typeParamsByName[tp.Name]; !exists {
			typeParamsByName[tp.Name] = tp
		}
	}
	if receiver != nil && sig.OwnerType != "" {
		for name, typ := range classTypeSubst(receiver, ownerTypeParams) {
			if typ == nil {
				continue
			}
			subst[name] = CloneTypeRef(typ)
		}
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
		} else if len(params) > 0 && params[len(params)-1] != nil && params[len(params)-1].Variadic {
			param = params[len(params)-1]
		} else if sig.Variadic {
			// C-style variadics (`...`) accept unconstrained trailing args.
			param = nil
		}

		var expectedArg *tokens.TypeRef
		if param != nil {
			expectedArg = SubstituteTypeParams(param.Type, subst)
		}
		actual := a.inferArgumentType(arg, env, expectedArg)
		if param != nil && param.Type != nil {
			if err := unifyType(param.Type, actual, subst, typeParamsByName); err != nil {
				concreteExpected := SubstituteTypeParams(param.Type, subst)
				if concreteExpected != nil && actual != nil && TypesCompatible(concreteExpected, actual) {
					// Allow backend-compatible argument coercions (for example string vs string*!).
					goto argCompatible
				}
				if concreteExpected != nil && actual != nil && !isUnresolvedTypeParamRef(concreteExpected, typeParamsByName) && !TypesCompatible(concreteExpected, actual) {
					a.program.addDiagnostic(Diagnostic{
						Severity: SeverityError,
						Kind:     DiagnosticTypeMismatch,
						Title:    "Type Mismatch",
						Message:  fmt.Sprintf("Function '%s' expects type '%s', got '%s'", sig.Name, TypeRefString(concreteExpected), TypeRefString(actual)),
						Pos:      arg.Pos,
					})
				}
				return nil
			}
		}
	argCompatible:
		concreteExpected := expectedArg
		if param != nil && param.Type != nil {
			concreteExpected = SubstituteTypeParams(param.Type, subst)
		}
		if concreteExpected != nil && actual != nil && !isUnresolvedTypeParamRef(concreteExpected, typeParamsByName) && !TypesCompatible(concreteExpected, actual) {
			a.program.addDiagnostic(Diagnostic{
				Severity: SeverityError,
				Kind:     DiagnosticTypeMismatch,
				Title:    "Type Mismatch",
				Message:  fmt.Sprintf("Function '%s' expects type '%s', got '%s'", sig.Name, TypeRefString(concreteExpected), TypeRefString(actual)),
				Pos:      arg.Pos,
			})
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
					Title:    "Trait Constraint Error",
					Message:  fmt.Sprintf("Type '%s' does not satisfy constraint '%s' for type parameter '%s'", TypeRefString(resolved), required, tp.Name),
				})
				return nil
			}
		}
	}

	ret := SubstituteTypeParams(sig.ReturnType, subst)
	ret = resolveSelfType(ret, sig.OwnerType)
	if expected != nil && ret != nil && !TypesCompatible(expected, ret) {
		return nil
	}
	return &resolutionAttempt{signature: sig, returnTyp: ret, subst: subst}
}
