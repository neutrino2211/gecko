// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// tryTempCounter generates unique names for try temporaries
var tryTempCounter int = 0

// getCurrentFuncReturnType walks up the scope hierarchy to find the current function's return type
func getCurrentFuncReturnType(scope *ast.Ast) *tokens.TypeRef {
	currentScope := scope
	for currentScope != nil {
		info, ok := (*CScopeDataMap)[currentScope.GetFullName()]
		if ok && info.CurrentFuncReturnType != nil {
			return info.CurrentFuncReturnType
		}
		currentScope = currentScope.Parent
	}
	return nil
}

// GetTryOperatorCall generates code for the 'try' operator with early return semantics
func (impl *CBackendImplementation) GetTryOperatorCall(operandCode string, operandType *tokens.TypeRef, operandExpr *tokens.Unary, scope *ast.Ast, pos lexer.Position) (string, bool) {
	if operandType == nil || operandType.Type == "" {
		return "", false
	}

	// Determine enclosing function return type up front; used both for validation
	// and to recover missing generic args in operand type inference.
	funcReturnType := getCurrentFuncReturnType(scope)

	// If type inference lost generic args (e.g., Option<File> -> Option),
	// recover the full return type from the called function symbol.
	if len(operandType.TypeArgs) == 0 {
		if parenIdx := strings.Index(operandCode, "("); parenIdx > 0 {
			callee := strings.TrimSpace(operandCode[:parenIdx])
			if fullType, ok := MethodReturnTypes[callee]; ok && fullType != nil {
				if fullType.Type == operandType.Type && len(fullType.TypeArgs) > 0 {
					operandType = fullType
				}
			}
			for key, fullType := range MethodReturnTypes {
				if (key != callee && !strings.HasSuffix(key, "#"+callee)) || fullType == nil {
					continue
				}
				if fullType.Type == operandType.Type && len(fullType.TypeArgs) > 0 {
					operandType = fullType
					break
				}
			}
		}
	}

	// Get the try hook (requires both has_value and try_unwrap methods)
	tryHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookTry)
	if tryHook == nil || len(tryHook.Methods) < 2 {
		return "", false
	}

	baseTypeName := operandType.Type
	typeName := baseTypeName

	// Collect C type args for mangling
	var typeArgStrs []string
	if len(operandType.TypeArgs) > 0 {
		typeArgStrs = make([]string, len(operandType.TypeArgs))
		for i, arg := range operandType.TypeArgs {
			if cType, ok := GeckoToCType[arg.Type]; ok {
				typeArgStrs[i] = cType
			} else {
				typeArgStrs[i] = arg.Type
			}
		}
		typeName = mangleName(baseTypeName, typeArgStrs)
	}

	// Check if the operand type implements Tryable
	mangledTraitName, found := impl.GetOperatorTraitName(typeName, tryHook.TraitName, scope)
	if !found {
		return "", false
	}
	mangledTraitName = concretizeGenericTraitName(mangledTraitName, baseTypeName, typeArgStrs)

	// Check if the current function's return type implements Tryable
	if funcReturnType == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Try Expression Error",
			"'try' can only be used inside a function",
			pos,
		)
		return "", false
	}

	// Build the return type name for trait lookup
	returnTypeName := funcReturnType.Type
	var returnTypeArgStrs []string
	if len(funcReturnType.TypeArgs) > 0 {
		returnTypeArgStrs = make([]string, len(funcReturnType.TypeArgs))
		for i, arg := range funcReturnType.TypeArgs {
			if cType, ok := GeckoToCType[arg.Type]; ok {
				returnTypeArgStrs[i] = cType
			} else {
				returnTypeArgStrs[i] = arg.Type
			}
		}
		returnTypeName = mangleName(funcReturnType.Type, returnTypeArgStrs)
	}

	// Check if the function's return type implements Tryable
	_, returnTypeHasTryable := impl.GetOperatorTraitName(returnTypeName, tryHook.TraitName, scope)

	// Build method names: has_value and try_unwrap
	hasValueMethod := tryHook.Methods[0]  // has_value
	tryUnwrapMethod := tryHook.Methods[1] // try_unwrap

	// Build method prefix (with or without module prefix for generic types)
	var methodPrefix string
	if len(typeArgStrs) > 0 {
		modulePrefix := getTypeOriginModule(typeName, scope)
		if modulePrefix != "" {
			modulePrefix += "__"
		}
		methodPrefix = modulePrefix + typeName + "__" + mangledTraitName + "__"
	} else {
		methodPrefix = typeName + "__" + mangledTraitName + "__"
	}

	hasValueCall := methodPrefix + hasValueMethod
	tryUnwrapCall := methodPrefix + tryUnwrapMethod

	// Get the C type for the operand (use mangled typeName for generic types)
	cTypeName := typeName

	// Generate unique temp variable name
	tryTempCounter++
	tempVar := "__try_tmp_" + strconv.Itoa(tryTempCounter)

	// Some inference paths still lose generic args for stdlib Option/Result.
	// In that case, fall back to field-based access with __auto_type.
	useStdTryFieldFallback := len(typeArgStrs) == 0 && (baseTypeName == "Option" || baseTypeName == "Result")
	hasValueExpr := hasValueCall + "(&" + tempVar + ")"
	tryUnwrapExpr := tryUnwrapCall + "(&" + tempVar + ")"
	tempDeclType := cTypeName
	if useStdTryFieldFallback {
		tempDeclType = "__auto_type"
		if baseTypeName == "Option" {
			hasValueExpr = tempVar + ".has_value"
		} else {
			hasValueExpr = tempVar + ".is_ok"
		}
		tryUnwrapExpr = tempVar + ".value"
	}

	// Decide failure behavior:
	// - If enclosing function returns a compatible Tryable, propagate.
	// - Otherwise, trap (panic ergonomics for non-Tryable returns).
	sourceFile := pos.Filename
	if sourceFile == "" && scope != nil {
		sourceFile = scope.GetSourceFile()
	}
	fileLit := strconv.Quote(sourceFile)
	geckoExpr := strings.TrimSpace(geckoUnaryToString(operandExpr))
	if geckoExpr == "" {
		geckoExpr = strings.TrimSpace(operandCode)
	}
	exprLit := strconv.Quote(geckoExpr)
	tryFailCall := "__gecko_try_fail(" + fileLit + ", " + strconv.Itoa(pos.Line) + ", " + strconv.Itoa(pos.Column) + ", __func__, " + exprLit + ");"
	failureAction := tryFailCall
	if returnTypeHasTryable {
		// Exact same concrete type: propagate directly.
		if returnTypeName == typeName {
			failureAction = "return " + tempVar + ";"
		} else if baseTypeName == "Option" && funcReturnType.Type == "Option" {
			// Option<T1> tried in Option<T2> function: propagate None as Option<T2>::none().
			returnModulePrefix := getTypeOriginModule(returnTypeName, scope)
			if returnModulePrefix != "" {
				returnModulePrefix += "__"
			}
			failureAction = "return " + returnModulePrefix + returnTypeName + "__none();"
		} else if baseTypeName == "Result" && funcReturnType.Type == "Result" &&
			len(operandType.TypeArgs) >= 2 && len(funcReturnType.TypeArgs) >= 2 {
			// Result<T1, E> tried in Result<T2, E>: propagate Err by re-wrapping error.
			operandErrType := TypeRefToCType(operandType.TypeArgs[1], scope)
			returnErrType := TypeRefToCType(funcReturnType.TypeArgs[1], scope)
			if operandErrType == returnErrType {
				returnModulePrefix := getTypeOriginModule(returnTypeName, scope)
				if returnModulePrefix != "" {
					returnModulePrefix += "__"
				}
				failureAction = "return " + returnModulePrefix + returnTypeName + "__err(" + tempVar + ".error);"
			}
		}
	}
	if strings.HasPrefix(failureAction, "return ") {
		cleanup := &CScopeInformation{}
		impl.cleanupThrough(scope, cleanup, false)
		failureAction = cleanup.Code + failureAction
	}

	// Generate GCC statement expression with early return:
	// ({ Type tmp = expr; if (!has_value(&tmp)) { ... } try_unwrap(&tmp); })
	operandCode = impl.moveUnaryValue(operandExpr, operandCode, scope)
	code := "({ " + tempDeclType + " " + tempVar + " = " + operandCode + "; " +
		"if (!(" + hasValueExpr + ")) { " + failureAction + " } " +
		tryUnwrapExpr + "; })"

	return code, true
}
