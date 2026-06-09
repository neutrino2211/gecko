// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// blockCounter is used to generate unique block names
var blockCounter uint64

func getUniqueBlockID() uint64 {
	return atomic.AddUint64(&blockCounter, 1)
}

func (impl *LLVMBackendImplementation) ensureConditionValue(scope *ast.Ast, cond value.Value, pos lexer.Position) value.Value {
	if cond == nil {
		return nil
	}
	if intType, ok := cond.Type().(*types.IntType); ok {
		if intType.BitSize == 1 {
			return cond
		}
		info := LLVMGetScopeInformation(scope)
		return info.LocalContext.MainBlock.NewICmp(enum.IPredNE, cond, constant.NewInt(intType, 0))
	}
	if ptrType, ok := cond.Type().(*types.PointerType); ok {
		info := LLVMGetScopeInformation(scope)
		return info.LocalContext.MainBlock.NewICmp(enum.IPredNE, cond, constant.NewNull(ptrType))
	}
	scope.ErrorScope.NewCompileTimeError("Control Flow Error", "condition expression must evaluate to bool/integer/pointer", pos)
	return nil
}

// NewIf generates LLVM IR for if/else-if/else statements
func (impl *LLVMBackendImplementation) NewIf(scope *ast.Ast, ifStmt *tokens.If) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "if statement must be inside a function", ifStmt.Pos)
		return
	}

	// Generate the condition value
	condValue := impl.ExpressionToLLIRValue(ifStmt.Expression, scope, &tokens.TypeRef{})
	condValue = impl.ensureConditionValue(scope, condValue, ifStmt.Pos)
	if condValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate if condition", ifStmt.Pos)
		return
	}

	// Get the function for creating blocks
	fn := info.LocalContext.Func
	blockID := getUniqueBlockID()

	// Create the basic blocks
	thenBlock := fn.NewBlock(fmt.Sprintf("if.then.%d", blockID))
	mergeBlock := fn.NewBlock(fmt.Sprintf("if.merge.%d", blockID))

	// Handle else-if and else chains
	var elseBlock *ir.Block
	if ifStmt.ElseIf != nil || ifStmt.Else != nil {
		elseBlock = fn.NewBlock(fmt.Sprintf("if.else.%d", blockID))
	} else {
		elseBlock = mergeBlock
	}

	// Create conditional branch from current block
	info.LocalContext.MainBlock.NewCondBr(condValue, thenBlock, elseBlock)

	// Process the then block
	info.LocalContext.MainBlock = thenBlock
	impl.Backend.ProcessEntries(ifStmt.Value, scope)

	// Add unconditional branch to merge block if not already terminated
	if info.LocalContext.MainBlock.Term == nil {
		info.LocalContext.MainBlock.NewBr(mergeBlock)
	}

	// Process else-if chain
	if ifStmt.ElseIf != nil {
		info.LocalContext.MainBlock = elseBlock
		impl.processElseIf(scope, ifStmt.ElseIf, mergeBlock, blockID)
	} else if ifStmt.Else != nil {
		// Process else block
		info.LocalContext.MainBlock = elseBlock
		impl.Backend.ProcessEntries(ifStmt.Else.Value, scope)
		if info.LocalContext.MainBlock.Term == nil {
			info.LocalContext.MainBlock.NewBr(mergeBlock)
		}
	}

	// Continue with merge block
	info.LocalContext.MainBlock = mergeBlock
}

// processElseIf handles else-if chains recursively
func (impl *LLVMBackendImplementation) processElseIf(scope *ast.Ast, elseIf *tokens.ElseIf, mergeBlock *ir.Block, parentBlockID uint64) {
	info := LLVMGetScopeInformation(scope)
	fn := info.LocalContext.Func

	// Generate the condition value for else-if
	condValue := impl.ExpressionToLLIRValue(elseIf.Expression, scope, &tokens.TypeRef{})
	condValue = impl.ensureConditionValue(scope, condValue, elseIf.Pos)
	if condValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate else-if condition", elseIf.Pos)
		return
	}

	blockID := getUniqueBlockID()

	// Create the then block for this else-if
	thenBlock := fn.NewBlock(fmt.Sprintf("elseif.then.%d", blockID))

	// Determine the else block
	var elseBlock *ir.Block
	if elseIf.ElseIf != nil || elseIf.Else != nil {
		elseBlock = fn.NewBlock(fmt.Sprintf("elseif.else.%d", blockID))
	} else {
		elseBlock = mergeBlock
	}

	// Create conditional branch
	info.LocalContext.MainBlock.NewCondBr(condValue, thenBlock, elseBlock)

	// Process the then block
	info.LocalContext.MainBlock = thenBlock
	impl.Backend.ProcessEntries(elseIf.Value, scope)
	if info.LocalContext.MainBlock.Term == nil {
		info.LocalContext.MainBlock.NewBr(mergeBlock)
	}

	// Process nested else-if or else
	if elseIf.ElseIf != nil {
		info.LocalContext.MainBlock = elseBlock
		impl.processElseIf(scope, elseIf.ElseIf, mergeBlock, blockID)
	} else if elseIf.Else != nil {
		info.LocalContext.MainBlock = elseBlock
		impl.Backend.ProcessEntries(elseIf.Else.Value, scope)
		if info.LocalContext.MainBlock.Term == nil {
			info.LocalContext.MainBlock.NewBr(mergeBlock)
		}
	}
}

// NewLoop generates LLVM IR for loop statements
func (impl *LLVMBackendImplementation) NewLoop(scope *ast.Ast, loop *tokens.Loop) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "loop statement must be inside a function", loop.Pos)
		return
	}

	// Get the function for creating blocks
	fn := info.LocalContext.Func
	blockID := getUniqueBlockID()

	// Handle different loop types
	if loop.ForExpression != nil || loop.WhileExpr != nil {
		// Simple while-style loop: for (condition) { body } or while condition { body }
		impl.processWhileLoop(scope, loop, fn, blockID)
	} else if loop.ForOf != nil {
		// For-of loop: for (let x of array) { body }
		impl.processForOfLoop(scope, loop, fn, blockID)
	} else if loop.ForIn != nil {
		// For-in loop: for (let i in array) { body }
		impl.processForInLoop(scope, loop, fn, blockID)
	}
}

// processWhileLoop handles simple expression-based loops (while-style)
func (impl *LLVMBackendImplementation) processWhileLoop(scope *ast.Ast, loop *tokens.Loop, fn *ir.Func, blockID uint64) {
	info := LLVMGetScopeInformation(scope)

	// Create the basic blocks for the loop
	headerBlock := fn.NewBlock(fmt.Sprintf("loop.header.%d", blockID))
	bodyBlock := fn.NewBlock(fmt.Sprintf("loop.body.%d", blockID))
	exitBlock := fn.NewBlock(fmt.Sprintf("loop.exit.%d", blockID))

	// Branch from current block to loop header
	info.LocalContext.MainBlock.NewBr(headerBlock)

	// Generate the condition in the header block
	info.LocalContext.MainBlock = headerBlock
	condExpr := loop.ForExpression
	if condExpr == nil {
		condExpr = loop.WhileExpr
	}
	condValue := impl.ExpressionToLLIRValue(condExpr, scope, &tokens.TypeRef{})
	condValue = impl.ensureConditionValue(scope, condValue, loop.Pos)
	if condValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate loop condition", loop.Pos)
		return
	}

	// Conditional branch: if condition is true, go to body; otherwise exit
	headerBlock.NewCondBr(condValue, bodyBlock, exitBlock)

	// Process the loop body
	info.LocalContext.MainBlock = bodyBlock
	info.LocalContext.PushLoopTargets(exitBlock, headerBlock)
	impl.Backend.ProcessEntries(loop.Value, scope)
	info.LocalContext.PopLoopTargets()

	// Add back-edge to header if not already terminated
	if info.LocalContext.MainBlock.Term == nil {
		info.LocalContext.MainBlock.NewBr(headerBlock)
	}

	// Continue with exit block
	info.LocalContext.MainBlock = exitBlock
}

// processForOfLoop handles for-of style loops (iterating over values)
func (impl *LLVMBackendImplementation) processForOfLoop(scope *ast.Ast, loop *tokens.Loop, fn *ir.Func, blockID uint64) {
	info := LLVMGetScopeInformation(scope)

	// Create the basic blocks for the loop
	headerBlock := fn.NewBlock(fmt.Sprintf("forof.header.%d", blockID))
	bodyBlock := fn.NewBlock(fmt.Sprintf("forof.body.%d", blockID))
	exitBlock := fn.NewBlock(fmt.Sprintf("forof.exit.%d", blockID))

	// Get the source array/iterable
	sourceValue := impl.ExpressionToLLIRValue(loop.ForOf.SourceArray, scope, &tokens.TypeRef{})
	if sourceValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate source array in for-of loop", loop.Pos)
		return
	}

	// Create index variable (i64 starting at 0)
	indexPtr := info.LocalContext.MainBlock.NewAlloca(types.I64)
	info.LocalContext.MainBlock.NewStore(constant.NewInt(types.I64, 0), indexPtr)

	// Branch to header
	info.LocalContext.MainBlock.NewBr(headerBlock)

	// Header block: check if index < length
	info.LocalContext.MainBlock = headerBlock

	// Load current index
	currentIndex := headerBlock.NewLoad(types.I64, indexPtr)

	// For now, we'll need the array length - this is a simplified version
	// In a full implementation, you'd get the actual array length
	// For now, we'll assume arrays have a known length or use a sentinel
	// This is a placeholder that should be enhanced with proper array length handling
	arrayLength := constant.NewInt(types.I64, 0) // Placeholder - needs proper implementation

	// Compare index < length
	cond := headerBlock.NewICmp(equalityOps["!="], currentIndex, arrayLength)
	headerBlock.NewCondBr(cond, bodyBlock, exitBlock)

	// Body block
	info.LocalContext.MainBlock = bodyBlock

	// Create the loop variable and assign it the current element
	if loop.ForOf.Variable != nil {
		impl.NewVariable(scope, loop.ForOf.Variable)
	}

	// Process loop body
	info.LocalContext.PushLoopTargets(exitBlock, headerBlock)
	impl.Backend.ProcessEntries(loop.Value, scope)
	info.LocalContext.PopLoopTargets()

	// Increment index
	if info.LocalContext.MainBlock.Term == nil {
		newIndex := info.LocalContext.MainBlock.NewAdd(currentIndex, constant.NewInt(types.I64, 1))
		info.LocalContext.MainBlock.NewStore(newIndex, indexPtr)
		info.LocalContext.MainBlock.NewBr(headerBlock)
	}

	// Continue with exit block
	info.LocalContext.MainBlock = exitBlock
}

// processForInLoop handles for-in style loops (iterating over indices)
func (impl *LLVMBackendImplementation) processForInLoop(scope *ast.Ast, loop *tokens.Loop, fn *ir.Func, blockID uint64) {
	info := LLVMGetScopeInformation(scope)

	// Create the basic blocks for the loop
	headerBlock := fn.NewBlock(fmt.Sprintf("forin.header.%d", blockID))
	bodyBlock := fn.NewBlock(fmt.Sprintf("forin.body.%d", blockID))
	exitBlock := fn.NewBlock(fmt.Sprintf("forin.exit.%d", blockID))

	// Get the source range/array
	sourceValue := impl.ExpressionToLLIRValue(loop.ForIn.SourceArray, scope, &tokens.TypeRef{})
	if sourceValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate source in for-in loop", loop.Pos)
		return
	}

	// Create index variable (i64 starting at 0)
	indexPtr := info.LocalContext.MainBlock.NewAlloca(types.I64)
	info.LocalContext.MainBlock.NewStore(constant.NewInt(types.I64, 0), indexPtr)

	// Branch to header
	info.LocalContext.MainBlock.NewBr(headerBlock)

	// Header block: check if index < length
	info.LocalContext.MainBlock = headerBlock

	// Load current index
	currentIndex := headerBlock.NewLoad(types.I64, indexPtr)

	// Placeholder for array length - needs proper implementation
	arrayLength := constant.NewInt(types.I64, 0)

	// Compare index < length
	cond := headerBlock.NewICmp(equalityOps["!="], currentIndex, arrayLength)
	headerBlock.NewCondBr(cond, bodyBlock, exitBlock)

	// Body block
	info.LocalContext.MainBlock = bodyBlock

	// Create the index variable in scope
	if loop.ForIn.Variable != nil {
		// The variable should hold the current index
		impl.NewVariable(scope, loop.ForIn.Variable)
	}

	// Process loop body
	info.LocalContext.PushLoopTargets(exitBlock, headerBlock)
	impl.Backend.ProcessEntries(loop.Value, scope)
	info.LocalContext.PopLoopTargets()

	// Increment index
	if info.LocalContext.MainBlock.Term == nil {
		newIndex := info.LocalContext.MainBlock.NewAdd(currentIndex, constant.NewInt(types.I64, 1))
		info.LocalContext.MainBlock.NewStore(newIndex, indexPtr)
		info.LocalContext.MainBlock.NewBr(headerBlock)
	}

	// Continue with exit block
	info.LocalContext.MainBlock = exitBlock
}

// NewAsm handles inline assembly statements
func (impl *LLVMBackendImplementation) NewAsm(scope *ast.Ast, asm *tokens.Asm) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Inline Assembly Error", "asm statement must be inside a function", asm.Pos)
		return
	}

	// Strip quotes from the assembly code string (participle captures them)
	asmCode := asm.Code
	if len(asmCode) >= 2 && asmCode[0] == '"' && asmCode[len(asmCode)-1] == '"' {
		asmCode = asmCode[1 : len(asmCode)-1]
	}

	// Create a function type for the inline asm (void return, no params)
	funcType := types.NewFunc(types.Void)

	// Create inline assembly with pointer to function type
	// The Typ field must be a pointer to a function type for NewCall to work
	inlineAsm := ir.NewInlineAsm(types.NewPointer(funcType), asmCode, "")
	inlineAsm.SideEffect = true

	// Call the inline assembly
	info.LocalContext.MainBlock.NewCall(inlineAsm)
}

// NewAssignment handles variable assignment statements
func (impl *LLVMBackendImplementation) NewAssignment(scope *ast.Ast, assignment *tokens.Assignment) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "assignment must be inside a function", assignment.Pos)
		return
	}

	resolveScope := scope
	if assignment.Global {
		resolveScope = scope.GetRoot()
	}

	// Resolve the variable being assigned to
	variable := resolveScope.ResolveSymbolAsVariable(assignment.Name)
	if variable.IsNil() {
		if assignment.Global {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to resolve global variable '"+assignment.Name+"'", assignment.Pos)
		} else {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to resolve variable '"+assignment.Name+"'", assignment.Pos)
		}
		return
	}

	varInfo := LLVMGetValueInformation(variable.Unwrap())

	// Helper: given a base pointer, index expression, and volatility, compute the element
	// pointer via GEP and store the assigned value.
	// Handles all three cases:
	//   1. Pointer-typed variable (buffer: T*)       → load actual T*, GEP on loaded pointer
	//   2. Fixed-size array variable (buffer: [N]T)  → GEP(arr, base, 0, index)
	//   3. Non-pointer variable (buffer: T)           → GEP(T, base, index)
	assignToIndex := func(basePtr value.Value, indexExpr *tokens.Expression, isVolatile bool, geckoTypeOverride *tokens.TypeRef) {
		ptrType, isPtr := basePtr.Type().(*types.PointerType)
		if !isPtr || ptrType.ElemType == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "indexed assignment target is not addressable", assignment.Pos)
			return
		}

		elemType := ptrType.ElemType

		indexValue := impl.ExpressionToLLIRValue(indexExpr, scope, &tokens.TypeRef{Type: "uint64"})
		if indexValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate index expression", assignment.Pos)
			return
		}

		// Determine the element type ref for the value to assign
		elemTypeRef := &tokens.TypeRef{}
		if geckoTypeOverride != nil {
			elemTypeRef = geckoTypeOverride
		} else if varInfo.GeckoType != nil && varInfo.GeckoType.Array != nil {
			elemTypeRef = varInfo.GeckoType.Array
		} else if varInfo.GeckoType != nil && varInfo.GeckoType.Pointer {
			elemTypeRef = &tokens.TypeRef{
				Type:     varInfo.GeckoType.Type,
				Volatile: varInfo.GeckoType.Volatile,
				Const:    varInfo.GeckoType.Const,
			}
		}

		newValue := impl.ExpressionToLLIRValue(assignment.Value, scope, elemTypeRef)
		if newValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate assignment value", assignment.Pos)
			return
		}

		var elemPtr value.Value
		if innerPtrType, isPtrType := elemType.(*types.PointerType); isPtrType {
			// The base stores a pointer (e.g., T**, or a field containing T*).
			// Load the actual pointer value first, then GEP on the loaded pointer.
			loadedPtr := impl.NewVolatileLoad(info.LocalContext.MainBlock, elemType, basePtr, isVolatile)
			if loadedPtr != nil {
				elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(innerPtrType.ElemType, loadedPtr, indexValue)
			}
		} else if _, isArray := elemType.(*types.ArrayType); isArray {
			// Fixed-size array: use two-index GEP (0, index)
			zero := constant.NewInt(types.I64, 0)
			elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(elemType, basePtr, zero, indexValue)
		} else {
			// Scalar type: single-index GEP treats the base as array-of-T
			elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(elemType, basePtr, indexValue)
		}

		if elemPtr == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to compute element address for indexed assignment", assignment.Pos)
			return
		}

		store := impl.NewVolatileStore(info.LocalContext.MainBlock, newValue, elemPtr, isVolatile)
		if store == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "indexed assignment has incompatible value type", assignment.Pos)
		}
	}

	// Handle field assignment (e.g., obj.field = value or obj.field[i] = value)
	if assignment.Field != "" {
		chain := &tokens.ChainAccess{Name: assignment.Field}
		chain.Pos = assignment.Pos
		fieldResolveScope := scope
		if assignment.Global {
			fieldResolveScope = scope.GetRoot()
		}
		fieldPtr := impl.ResolveSymbolChainValue(fieldResolveScope, assignment.Name, []*tokens.ChainAccess{chain}, assignment.Pos, true)
		if fieldPtr == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to resolve field assignment target '"+assignment.Name+"."+assignment.Field+"'", assignment.Pos)
			return
		}

		ptrType, isPtr := fieldPtr.Type().(*types.PointerType)
		if !isPtr || ptrType.ElemType == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "field assignment target is not addressable", assignment.Pos)
			return
		}

		if assignment.Index != nil {
			// Field + index: self.buffer[i] = value
			// Derive the element type ref from the field's LLVM type
			fieldLLVMType := ptrType.ElemType
			fieldElemTypeRef := &tokens.TypeRef{}
			if innerPtr, isPtr := fieldLLVMType.(*types.PointerType); isPtr {
				fieldElemTypeRef = llirTypeToGeckoTypeRef(innerPtr.ElemType)
			} else if arrType, isArray := fieldLLVMType.(*types.ArrayType); isArray {
				fieldElemTypeRef = llirTypeToGeckoTypeRef(arrType.ElemType)
			} else {
				fieldElemTypeRef = llirTypeToGeckoTypeRef(fieldLLVMType)
			}
			isVolatile := false
			assignToIndex(fieldPtr, assignment.Index, isVolatile, fieldElemTypeRef)
			return
		}

		newValue := impl.ExpressionToLLIRValue(assignment.Value, scope, &tokens.TypeRef{})
		if newValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate assignment value", assignment.Pos)
			return
		}
		newValue = impl.coerceValueToType(newValue, ptrType.ElemType, scope, assignment.Pos)
		if newValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "field assignment has incompatible value type", assignment.Pos)
			return
		}

		if impl.NewVolatileStore(info.LocalContext.MainBlock, newValue, fieldPtr, false) == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to store field assignment value", assignment.Pos)
		}
		return
	}

	// Handle indexed assignment (e.g., arr[0] = value)
	if assignment.Index != nil {
		ptrValue := varInfo.Value
		if ptrValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "variable '"+assignment.Name+"' has no value", assignment.Pos)
			return
		}

		if _, isPtr := ptrValue.Type().(*types.PointerType); !isPtr {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "cannot index a non-pointer variable '"+assignment.Name+"'", assignment.Pos)
			return
		}

		isVolatile := varInfo.IsVolatile || (varInfo.GeckoType != nil && varInfo.GeckoType.IsVolatile())
		assignToIndex(ptrValue, assignment.Index, isVolatile, nil)
		return
	}

	// Generate the value to assign
	newValue := impl.ExpressionToLLIRValue(assignment.Value, scope, varInfo.GeckoType)
	if newValue == nil {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate assignment value", assignment.Pos)
		return
	}

	if varInfo.Value == nil {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "variable '"+assignment.Name+"' has no storage for assignment", assignment.Pos)
		return
	}

	// Normal variable assignment stores into the variable's backing storage.
	// For locals/globals this is typically an alloca/global pointer.
	if dstPtr, isPtr := varInfo.Value.Type().(*types.PointerType); isPtr && dstPtr.ElemType != nil {
		newValue = impl.coerceValueToType(newValue, dstPtr.ElemType, scope, assignment.Pos)
		if newValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "assignment has incompatible value type", assignment.Pos)
			return
		}
		isVolatile := varInfo.IsVolatile || (varInfo.GeckoType != nil && varInfo.GeckoType.IsVolatile())
		if impl.NewVolatileStore(info.LocalContext.MainBlock, newValue, varInfo.Value, isVolatile) == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to store assignment value", assignment.Pos)
		}
		return
	}

	// Fallback for non-addressable values.
	varInfo.Value = newValue
}

func (impl *LLVMBackendImplementation) NewBreak(scope *ast.Ast) {
	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "break statement must be inside a function", lexer.Position{})
		return
	}

	breakTarget := info.LocalContext.CurrentLoopBreakTarget()
	if breakTarget == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "break statement must be inside a loop", lexer.Position{})
		return
	}

	if info.LocalContext.MainBlock.Term == nil {
		info.LocalContext.MainBlock.NewBr(breakTarget)
	}

	// Continue lowering subsequent statements into a dead block so IR remains structurally valid.
	dead := info.LocalContext.Func.NewBlock(fmt.Sprintf("loop.break.dead.%d", getUniqueBlockID()))
	info.LocalContext.MainBlock = dead
}

func (impl *LLVMBackendImplementation) NewContinue(scope *ast.Ast) {
	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "continue statement must be inside a function", lexer.Position{})
		return
	}

	continueTarget := info.LocalContext.CurrentLoopContinueTarget()
	if continueTarget == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "continue statement must be inside a loop", lexer.Position{})
		return
	}

	if info.LocalContext.MainBlock.Term == nil {
		info.LocalContext.MainBlock.NewBr(continueTarget)
	}

	// Continue lowering subsequent statements into a dead block so IR remains structurally valid.
	dead := info.LocalContext.Func.NewBlock(fmt.Sprintf("loop.continue.dead.%d", getUniqueBlockID()))
	info.LocalContext.MainBlock = dead
}

// NewCImport handles cimport statements (no-op for LLVM - linking handled externally)
func (impl *LLVMBackendImplementation) NewCImport(scope *ast.Ast, cimport *tokens.CImport) {
	// LLVM backend handles C interop through extern declarations, not includes
	// The WithObject/WithLibrary info can be used by the linker stage
}

func ensureLLVMForeignModuleScope(root *ast.Ast, moduleName string) *ast.Ast {
	if root == nil || moduleName == "" {
		return nil
	}
	if existing, ok := root.Children[moduleName]; ok && existing != nil {
		return existing
	}
	moduleScope := &ast.Ast{
		Scope:            moduleName,
		Parent:           nil,
		IsImportedModule: true,
		OriginModule:     root.GetRoot().Scope,
		SourceFile:       root.GetSourceFile(),
	}
	moduleScope.Init(root.ErrorScope)
	moduleScope.Config = root.Config
	root.Children[moduleName] = moduleScope
	return moduleScope
}

func unquoteIfQuoted(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func (impl *LLVMBackendImplementation) NewForeign(scope *ast.Ast, foreign *tokens.Foreign) {
	if scope == nil || foreign == nil {
		return
	}
	backendName := strings.TrimSpace(unquoteIfQuoted(foreign.Backend))
	if backendName == "" {
		backendName = "c"
	}
	if backendName != "c" {
		scope.ErrorScope.NewCompileTimeError(
			"Foreign Backend Error",
			fmt.Sprintf("unsupported foreign backend '%s' for LLVM backend (expected \"c\")", backendName),
			foreign.Pos,
		)
		return
	}

	rootScope := scope.GetRoot()
	moduleScope := ensureLLVMForeignModuleScope(rootScope, foreign.Module)
	if moduleScope == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Foreign Module Error",
			"unable to initialize foreign module scope",
			foreign.Pos,
		)
		return
	}

	for _, member := range foreign.Members {
		if member == nil || member.Type == nil {
			continue
		}
		impl.NewExternalType(rootScope, &tokens.ExternalType{Name: member.Type.Name})
		if classOpt := rootScope.ResolveClass(member.Type.Name); !classOpt.IsNil() {
			moduleScope.Classes[member.Type.Name] = classOpt.Unwrap()
		}
	}

	for _, member := range foreign.Members {
		if member == nil || member.Method == nil || member.Method.Name == "" {
			continue
		}
		method := &tokens.Method{
			Visibility: "external",
			Name:       member.Method.Name,
			Arguments:  member.Method.Arguments,
			Variadic:   member.Method.IsVariadic(),
			Type:       member.Method.Type,
			Throws:     member.Method.Throws,
			LinkName:   member.Method.As,
		}
		impl.NewMethod(moduleScope, method)
	}
}

// IntrinsicStatement handles intrinsic calls as statements
func (impl *LLVMBackendImplementation) IntrinsicStatement(scope *ast.Ast, i *tokens.Intrinsic) {
	impl.IntrinsicToLLIRValue(scope, i, &tokens.TypeRef{})
}
