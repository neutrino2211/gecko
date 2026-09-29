// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	geckoerrors "github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) inferFuncCall(call *tokens.FuncCall, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if call == nil {
		return nil
	}
	if env != nil {
		if receiver := env.lookup(call.Module); receiver != nil {
			a.recordOccurrence(receiver.SymbolID, call.Pos, call.Module, false)
		} else {
			a.recordModuleQualifier(call.Module, call.Pos)
		}
	}
	if call.StaticModule != "" {
		a.recordModuleQualifier(call.StaticModule, call.Pos)
	}
	if call.StaticType != "" {
		position := call.Pos
		if call.StaticModule != "" {
			position.Offset += len(call.StaticModule) + 1
			position.Column += len(call.StaticModule) + 1
		}
		staticType := &tokens.TypeRef{
			Module:   call.StaticModule,
			Type:     call.StaticType,
			TypeArgs: call.StaticTypeArgs,
		}
		staticType.Pos = position
		a.recordTypeRef(staticType)
	}
	for _, argument := range call.TypeArgs {
		a.recordTypeRef(argument)
	}
	var candidates []*FunctionSignature
	var receiver *tokens.TypeRef
	if call.Module != "" {
		receiver = a.lookupReceiverType(call.Module, env)
	}
	if receiver != nil {
		candidates, receiver = a.methodCandidatesForReceiver(receiver, call.Function)
	} else {
		candidates = a.lookupCallCandidates(call)
	}
	if len(candidates) == 0 {
		a.program.funcCalls[call] = &CallResolution{Status: ResolutionIncomplete, ReceiverType: CloneTypeRef(receiver)}
		return nil
	}
	if call.StaticType != "" {
		receiver = &tokens.TypeRef{
			Type:     call.StaticType,
			TypeArgs: cloneTypeRefSlice(call.StaticTypeArgs),
		}
	}
	attempt := a.resolveCallCandidates(candidates, call.TypeArgs, call.Arguments, receiver, expected, env, call.Pos, call.EndPos.Offset)
	if attempt == nil {
		a.program.funcCalls[call] = &CallResolution{Status: ResolutionInvalid, ReceiverType: CloneTypeRef(receiver)}
		return nil
	}

	res := resolvedCall(attempt, receiver, len(call.TypeArgs) > 0)
	a.program.funcCalls[call] = res
	a.recordCallOccurrence(res.CalleeID, call.Pos, call.Function)
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
			id := a.classSymbolID(call.StaticType, "")
			if id == 0 {
				id = a.traitSymbolID(call.StaticType, "")
			}
			if id == 0 {
				if len(a.program.classSymbolIDs[call.StaticType])+len(a.program.traitSymbolIDs[call.StaticType]) > 1 {
					return nil
				}
				return append([]*FunctionSignature{}, cands...)
			}
			module := symbolModule(a.program.SymbolByID(id))
			filtered := make([]*FunctionSignature, 0, len(cands))
			for _, candidate := range cands {
				if candidate.Module == module {
					filtered = append(filtered, candidate)
				}
			}
			return filtered
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

func (a *analyzer) resolveCallCandidates(candidates []*FunctionSignature, explicitTypeArgs []*tokens.TypeRef, args []*tokens.Argument, receiver *tokens.TypeRef, expected *tokens.TypeRef, env *flowEnv, pos lexer.Position, endOffset int) *resolutionAttempt {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 && candidates[0] != nil && (candidates[0].OwnerType == "" || receiver != nil) && pos.Line > 0 {
		sig := candidates[0]
		params := callParameters(sig, receiver)
		if !callAcceptsArgumentCount(sig, params, len(args)) {
			expectedCount := len(params)
			if sig.Variadic {
				expectedCount = fixedParameterCount(params)
			}
			argumentName := "arguments"
			if expectedCount == 1 {
				argumentName = "argument"
			}
			message := fmt.Sprintf("Function '%s' expects %d %s, got %d", sig.Name, expectedCount, argumentName, len(args))
			if sig.Variadic {
				message = fmt.Sprintf("Variadic function '%s' expects at least %d %s, got %d", sig.Name, expectedCount, argumentName, len(args))
			}
			a.program.addDiagnostic(Diagnostic{
				Severity:  SeverityError,
				Kind:      DiagnosticArgumentCount,
				Title:     "Argument Count Mismatch",
				Message:   message,
				Pos:       pos,
				EndOffset: endOffset,
				Code:      geckoerrors.CodeArgumentCount,
			})
			return nil
		}
	}
	if len(candidates) == 1 {
		before := len(a.program.diagnostics)
		attempt := a.tryResolveCandidate(candidates[0], explicitTypeArgs, args, receiver, expected, env)
		if attempt != nil {
			attempt.diagnostics = append([]Diagnostic(nil), a.program.diagnostics[before:]...)
		}
		return attempt
	}
	type validCandidate struct {
		attempt *resolutionAttempt
		probe   *analyzer
	}
	valid := make([]validCandidate, 0)
	var failure []Diagnostic
	for _, cand := range candidates {
		if cand == nil {
			continue
		}
		probe := a.candidateProbe()
		attempt := probe.tryResolveCandidate(cand, explicitTypeArgs, args, receiver, expected, env)
		candidateDiagnostics := append([]Diagnostic(nil), probe.program.diagnostics[len(a.program.diagnostics):]...)
		if attempt != nil {
			attempt.diagnostics = candidateDiagnostics
			valid = append(valid, validCandidate{attempt: attempt, probe: probe})
		} else if len(failure) == 0 && len(candidateDiagnostics) > 0 {
			failure = candidateDiagnostics
		}
	}
	if len(valid) == 0 {
		for _, diagnostic := range failure {
			a.program.addDiagnostic(diagnostic)
		}
		return nil
	}
	if len(valid) > 1 {
		hasGenericCandidate := false
		for _, candidate := range valid {
			attempt := candidate.attempt
			if attempt != nil && attempt.signature != nil && len(attempt.signature.TypeParams) > 0 {
				hasGenericCandidate = true
				break
			}
		}
		if hasGenericCandidate {
			a.program.addDiagnostic(Diagnostic{
				Severity:  SeverityError,
				Kind:      DiagnosticInferenceAmbiguity,
				Title:     "Type Inference Ambiguity",
				Message:   "Multiple callable overloads matched; provide explicit type arguments to disambiguate",
				Help:      "Use explicit `<...>` type arguments on the call.",
				Pos:       pos,
				EndOffset: endOffset,
			})
			return nil
		}
	}
	selected := valid[0]
	*a.program = *selected.probe.program
	a.nameForID = selected.probe.nameForID
	a.setupNames = selected.probe.setupNames
	a.unsafeBlockCounter = selected.probe.unsafeBlockCounter
	return selected.attempt
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
	oldSelfType := a.selfType
	if sig.OwnerType != "" {
		a.selfType = sig.OwnerType
	}
	defer func() {
		a.selfType = oldSelfType
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
		}
	}

	params := callParameters(sig, receiver)
	if !callAcceptsArgumentCount(sig, params, len(args)) {
		return nil
	}

	bindings := make([]CallArgumentBinding, 0, len(args))
	for i, arg := range args {
		binding := CallArgumentBinding{ArgumentIndex: i, ParameterIndex: -1}
		if arg == nil {
			bindings = append(bindings, binding)
			continue
		}
		var param *tokens.Value
		if i < len(params) {
			param = params[i]
			binding.ParameterIndex = i
		} else if len(params) > 0 && params[len(params)-1] != nil && params[len(params)-1].Variadic {
			param = params[len(params)-1]
			binding.ParameterIndex = len(params) - 1
		} else if sig.Variadic {
			// C-style variadics (`...`) accept unconstrained trailing args.
			param = nil
		}
		if receiver != nil && len(sig.Params) > 0 && sig.Params[0] != nil && sig.Params[0].Name == "self" && binding.ParameterIndex >= 0 {
			binding.ParameterIndex++
		}
		if param != nil {
			binding.ParameterName = param.Name
		}

		var expectedArg *tokens.TypeRef
		if param != nil {
			expectedArg = SubstituteTypeParams(param.Type, subst)
		}
		actual := a.inferArgumentType(arg, env, expectedArg)
		binding.ActualType = CloneTypeRef(actual)
		if param != nil && param.Type != nil {
			if err := unifyType(param.Type, actual, subst, typeParamsByName); err != nil {
				concreteExpected := SubstituteTypeParams(param.Type, subst)
				if concreteExpected != nil && actual != nil && a.typesCompatible(concreteExpected, actual) {
					// Allow backend-compatible argument coercions (for example string vs string*!).
					goto argCompatible
				}
				if concreteExpected != nil && actual != nil && !isUnresolvedTypeParamRef(concreteExpected, typeParamsByName) && !a.typesCompatible(concreteExpected, actual) {
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
		if concreteExpected != nil && actual != nil && !isUnresolvedTypeParamRef(concreteExpected, typeParamsByName) && !a.typesCompatible(concreteExpected, actual) {
			a.program.addDiagnostic(Diagnostic{
				Severity: SeverityError,
				Kind:     DiagnosticTypeMismatch,
				Title:    "Type Mismatch",
				Message:  fmt.Sprintf("Function '%s' expects type '%s', got '%s'", sig.Name, TypeRefString(concreteExpected), TypeRefString(actual)),
				Pos:      arg.Pos,
			})
			return nil
		}
		binding.ExpectedType = CloneTypeRef(concreteExpected)
		bindings = append(bindings, binding)
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
	if expected != nil && ret != nil && !a.typesCompatible(expected, ret) {
		return nil
	}
	return &resolutionAttempt{signature: sig, returnTyp: ret, subst: subst, arguments: bindings}
}

func callParameters(sig *FunctionSignature, receiver *tokens.TypeRef) []*tokens.Value {
	if receiver != nil && len(sig.Params) > 0 && sig.Params[0] != nil && sig.Params[0].Name == "self" {
		return sig.Params[1:]
	}
	return sig.Params
}

func fixedParameterCount(params []*tokens.Value) int {
	for index, param := range params {
		if param != nil && param.Variadic {
			return index
		}
	}
	return len(params)
}

func callAcceptsArgumentCount(sig *FunctionSignature, params []*tokens.Value, count int) bool {
	return count >= fixedParameterCount(params) && (sig.Variadic || count <= len(params))
}
