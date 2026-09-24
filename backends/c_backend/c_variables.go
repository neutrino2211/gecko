// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewVariable generates a C variable declaration
func (impl *CBackendImplementation) NewVariable(scope *ast.Ast, f *tokens.Field) {
	info := CGetScopeInformation(scope)

	// `@unsafe with ... { }` as an initializer: lower the block as a statement
	// that yields its Result<T, E> into `f.Name` (e.g. `let r = @unsafe ...`).
	// The block itself declares and assigns the variable, so we skip the normal
	// `let` emission.
	if ub := f.Value.GetUnsafeBlock(); ub != nil {
		tokens.UnsafeBlockBindNames[ub] = f.Name
		impl.NewUnsafeBlock(scope, ub)
		return
	}

	// Track whether type was explicit or inferred
	typeWasExplicit := f.Type != nil

	// Type inference: if no explicit type, try to infer from the value
	if f.Type == nil {
		if f.Value != nil {
			if CurrentSemanticProgram != nil {
				f.Type = CurrentSemanticProgram.TypeOfExpression(f.Value)
			}

			// Create a symbol resolver that looks up variables in scope
			resolveSymbol := func(name string) *tokens.TypeRef {
				opt := scope.ResolveSymbolAsVariable(name)
				if !opt.IsNil() {
					v := opt.Unwrap()
					fullName := v.GetFullName()
					if valInfo, ok := (*CProgramValues)[fullName]; ok {
						return valInfo.GeckoType
					}
				}
				return nil
			}

			if f.Type == nil {
				f.Type = tokens.InferType(f.Value, resolveSymbol)
			}

			// If tokens.InferType returns nil, try backend's type inference
			// This handles function calls, method calls, etc.
			if f.Type == nil {
				f.Type = impl.GetTypeOfExpression(f.Value, scope)
			}
		}

		// If still nil, error
		if f.Type == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Type Inference Error",
				"Unable to infer variable type; please provide an explicit type annotation",
				f.Pos,
			)
			f.Type = &tokens.TypeRef{Type: "int"}
		}
	}

	// Check type visibility for explicit type annotations
	// (Inferred types already passed visibility when they were originally defined)
	if typeWasExplicit && f.Type != nil {
		f.Type.Check(scope)
	}

	// Validate explicit initializer type against declared type.
	if typeWasExplicit && f.Value != nil {
		impl.CheckVariableInitType(f, scope)
	}

	isConst := bindingIsConst(f)

	if f.Value == nil && isConst {
		scope.ErrorScope.NewCompileTimeError("Uninitialized Constant", "Constant must be initialized with a value", f.Pos)
		return
	}

	cType := TypeRefToCType(f.Type, scope)

	// Check if this is a global variable (no current function context)
	isGlobal := info.CurrentFunc == ""

	// Register variable in AST
	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    isConst,
		IsVolatile: f.Type.Volatile,
		IsPointer:  f.Type.Pointer,
		IsExternal: f.Visibility == "external",
		IsGlobal:   isGlobal,
		Parent:     scope,
	}

	// For globals, use the full qualified name; for locals, use just the name
	var varName string
	if isGlobal {
		varName = fieldVariable.GetFullName()
	} else {
		varName = f.Name
		if info.LexicalBlock {
			varName = fieldVariable.GetFullName()
			fieldVariable.CName = varName
		}
	}

	(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
		CType:     cType,
		GeckoType: f.Type,
	}

	if !isGlobal && dropMethodForType(f.Type, scope) != "" {
		fieldVariable.DropFlag = "__owned_" + fieldVariable.GetFullName()
	}
	scope.Variables[f.Name] = fieldVariable

	if isGlobal {
		// Generate global variable declaration with attributes
		impl.NewGlobalVariable(scope, f, cType, varName)
	} else {
		// Generate local variable declaration
		impl.NewLocalVariable(scope, f, cType, varName)
		info.LocalVarOrder = append(info.LocalVarOrder, f.Name)
		if fieldVariable.DropFlag != "" {
			info.Code += "int " + fieldVariable.DropFlag + " = 1;\n"
		}
	}

	// Apply move semantics only after RHS codegen has consumed the source.
	if f.Value != nil {
		impl.ApplyMoveFromExpression(f.Value, scope)
	}
}

// NewGlobalVariable generates a global C variable declaration with optional attributes
func (impl *CBackendImplementation) NewGlobalVariable(scope *ast.Ast, f *tokens.Field, cType string, varName string) {
	info := CGetScopeInformation(scope)

	// Build attributes string
	attrStr := tokens.ToCAttributes(f.Attributes)

	// Handle const modifier - either from Type.Const, from Mutability == "const",
	// or from the inner type's Const for sized arrays
	isConst := emitBindingConstPrefix(f)
	typeDecl := cType
	if isConst {
		typeDecl = "const " + typeDecl
	}

	// Handle sized arrays: for [N]T, we need to generate "T name[N]" format
	var varDecl string
	if f.Type.Size != nil {
		// Sized array: e.g., [4096]uint8 -> uint8_t name[4096]
		baseType := TypeRefToCType(f.Type.Size.Type, scope)
		if isConst {
			baseType = "const " + baseType
		}
		if attrStr != "" {
			varDecl = fmt.Sprintf("%s %s %s[%s]", attrStr, baseType, varName, f.Type.Size.Size)
		} else {
			varDecl = fmt.Sprintf("%s %s[%s]", baseType, varName, f.Type.Size.Size)
		}
	} else {
		// Regular type
		if attrStr != "" {
			varDecl = fmt.Sprintf("%s %s %s", attrStr, typeDecl, varName)
		} else {
			varDecl = fmt.Sprintf("%s %s", typeDecl, varName)
		}
	}

	// Add initializer if present
	if f.Value != nil {
		val := impl.ExpressionToCString(f.Value, scope)
		varDecl += " = " + val
	}

	varDecl += ";"

	info.Globals = append(info.Globals, varDecl)
}

// NewLocalVariable generates a local C variable declaration
func (impl *CBackendImplementation) NewLocalVariable(scope *ast.Ast, f *tokens.Field, cType string, varName string) {
	info := CGetScopeInformation(scope)

	// Infer lambda param types from variable's declared type
	if f.Value != nil && f.Type != nil && f.Type.FuncType != nil {
		if lambda := f.Value.GetLambda(); lambda != nil {
			InferLambdaTypes(lambda, f.Type.FuncType)
		}
	}

	// Binding const / non-pointer readonly; type-level readonly* is emitted via TypeRefToCType
	isConst := emitBindingConstPrefix(f)
	typeDecl := cType
	if isConst {
		typeDecl = "const " + typeDecl
	}

	// Generate variable declaration/definition
	var varDecl string

	// Handle function pointer types
	if IsFuncPointerType(cType) {
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			// Emit capture initialization for lambdas (must be after ExpressionToCString)
			if lambda := f.Value.GetLambda(); lambda != nil {
				key := fmt.Sprintf("%d:%d", lambda.Pos.Line, lambda.Pos.Column)
				if initCode, ok := PendingCaptureInit[key]; ok && initCode != "" {
					info.Code += initCode
					delete(PendingCaptureInit, key)
				}
			}
			varDecl = fmt.Sprintf("    %s = %s;\n", FormatFuncPointerDecl(cType, varName), val)
		} else {
			varDecl = fmt.Sprintf("    %s;\n", FormatFuncPointerDecl(cType, varName))
		}
	} else if f.Type.Size != nil {
		// Handle sized arrays: for [N]T, we need to generate "T name[N]" format
		baseType := TypeRefToCType(f.Type.Size.Type, scope)
		if isConst {
			baseType = "const " + baseType
		}
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			varDecl = fmt.Sprintf("    %s %s[%s] = %s;\n", baseType, varName, f.Type.Size.Size, val)
		} else {
			varDecl = fmt.Sprintf("    %s %s[%s] = {0};\n", baseType, varName, f.Type.Size.Size)
		}
	} else {
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			varDecl = fmt.Sprintf("    %s %s = %s;\n", typeDecl, varName, val)
		} else {
			varDecl = fmt.Sprintf("    %s %s = {0};\n", typeDecl, varName)
		}
	}

	info.Code += varDecl
}

// DestructuringDeclaration handles `let Point { x, y } = p`, binding each named
// field to a fresh local variable initialized from the source value.
func (impl *CBackendImplementation) DestructuringDeclaration(scope *ast.Ast, d *tokens.DestructuringDeclaration) {
	if d == nil {
		return
	}
	info := CGetScopeInformation(scope)

	valueType := impl.GetTypeOfExpression(d.Value, scope)
	if valueType == nil && CurrentSemanticProgram != nil {
		valueType = CurrentSemanticProgram.TypeOfExpression(d.Value)
	}

	// Resolve the struct class so field types can be recovered.
	var structClass *ast.Ast
	if valueType != nil {
		if classOpt := scope.ResolveClass(normalizeTypeName(valueType.Type)); !classOpt.IsNil() {
			structClass = classOpt.Unwrap()
		} else if classOpt := scope.ResolveClass(valueType.Type); !classOpt.IsNil() {
			structClass = classOpt.Unwrap()
		}
	}

	valueStr := impl.ExpressionToCString(d.Value, scope)
	if valueType != nil {
		tempName := fmt.Sprintf("__destructure_%d_%d", d.Pos.Line, d.Pos.Column)
		info.Code += fmt.Sprintf("    %s %s = %s;\n", TypeRefToCType(valueType, scope), tempName, valueStr)

		for _, binding := range d.Bindings {
			if binding == nil {
				continue
			}
			target := binding.Target()
			if target == "" {
				continue
			}

			fieldType := destructuredFieldType(structClass, binding.Field)
			cFieldType := "int32_t"
			if fieldType != nil {
				cFieldType = TypeRefToCType(fieldType, scope)
			}

			localVar := ast.Variable{
				Name:      target,
				IsPointer: fieldType != nil && fieldType.Pointer,
				Parent:    scope,
			}
			scope.Variables[target] = localVar
			info.LocalVarOrder = append(info.LocalVarOrder, target)
			(*CProgramValues)[localVar.GetFullName()] = &CValueInformation{
				CType:     cFieldType,
				GeckoType: fieldType,
			}

			info.Code += fmt.Sprintf("    %s %s = %s.%s;\n", cFieldType, target, tempName, binding.Field)
		}
	} else {
		// Type unknown: fall back to a statement preserving the evaluation once.
		info.Code += fmt.Sprintf("    %s;\n", valueStr)
	}
}

// destructuredFieldType looks up the Gecko type of a class field, returning nil
// when it cannot be resolved (e.g. opaque or generic structs).
func destructuredFieldType(structClass *ast.Ast, field string) *tokens.TypeRef {
	if structClass == nil || field == "" {
		return nil
	}
	fieldVar, ok := structClass.Variables[field]
	if !ok {
		return nil
	}
	if valInfo, ok := (*CProgramValues)[fieldVar.GetFullName()]; ok && valInfo != nil && valInfo.GeckoType != nil {
		return valInfo.GeckoType
	}
	return nil
}
