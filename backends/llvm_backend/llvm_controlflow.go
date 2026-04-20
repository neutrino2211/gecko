package llvmbackend

import (
	"fmt"
	"sync/atomic"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// blockCounter is used to generate unique block names
var blockCounter uint64

func getUniqueBlockID() uint64 {
	return atomic.AddUint64(&blockCounter, 1)
}

// NewIf generates LLVM IR for if/else-if/else statements
func (impl *LLVMBackendImplementation) NewIf(scope *ast.Ast, ifStmt *tokens.If) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "if statement must be inside a function", ifStmt.Pos)
		return
	}

	// Generate the condition value
	condValue := impl.ExpressionToLLIRValue(ifStmt.Expression, scope, &tokens.TypeRef{Type: "bool"})
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
	condValue := impl.ExpressionToLLIRValue(elseIf.Expression, scope, &tokens.TypeRef{Type: "bool"})
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
	if loop.ForExpression != nil {
		// Simple while-style loop: for (condition) { body }
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
	condValue := impl.ExpressionToLLIRValue(loop.ForExpression, scope, &tokens.TypeRef{Type: "bool"})
	if condValue == nil {
		scope.ErrorScope.NewCompileTimeError("Control Flow Error", "unable to evaluate loop condition", loop.Pos)
		return
	}

	// Conditional branch: if condition is true, go to body; otherwise exit
	headerBlock.NewCondBr(condValue, bodyBlock, exitBlock)

	// Process the loop body
	info.LocalContext.MainBlock = bodyBlock
	impl.Backend.ProcessEntries(loop.Value, scope)

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
	impl.Backend.ProcessEntries(loop.Value, scope)

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
	impl.Backend.ProcessEntries(loop.Value, scope)

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

	// Resolve the variable being assigned to
	variable := scope.ResolveSymbolAsVariable(assignment.Name)
	if variable.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to resolve variable '"+assignment.Name+"'", assignment.Pos)
		return
	}

	varInfo := LLVMGetValueInformation(variable.Unwrap())

	// Handle indexed assignment (e.g., arr[0] = value)
	if assignment.Index != nil {
		// Get the pointer value of the variable
		ptrValue := varInfo.Value
		if ptrValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "variable '"+assignment.Name+"' has no value", assignment.Pos)
			return
		}

		// Get the pointer type
		ptrType, isPtr := ptrValue.Type().(*types.PointerType)
		if !isPtr {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "cannot index a non-pointer variable '"+assignment.Name+"'", assignment.Pos)
			return
		}

		elemType := ptrType.ElemType

		// Get the index value
		indexValue := impl.ExpressionToLLIRValue(assignment.Index, scope, &tokens.TypeRef{Type: "uint64"})
		if indexValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate index expression", assignment.Pos)
			return
		}

		// Get the element type for the value we're assigning
		elemTypeRef := &tokens.TypeRef{}
		if varInfo.GeckoType != nil && varInfo.GeckoType.Array != nil {
			elemTypeRef = varInfo.GeckoType.Array
		} else if varInfo.GeckoType != nil && varInfo.GeckoType.Pointer {
			// For pointer types, create a copy without the pointer flag
			elemTypeRef = &tokens.TypeRef{
				Type:     varInfo.GeckoType.Type,
				Volatile: varInfo.GeckoType.Volatile,
				Const:    varInfo.GeckoType.Const,
			}
		}

		// Generate the value to assign
		newValue := impl.ExpressionToLLIRValue(assignment.Value, scope, elemTypeRef)
		if newValue == nil {
			scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate assignment value", assignment.Pos)
			return
		}

		// Use getelementptr to get the address of the indexed element
		elemPtr := info.LocalContext.MainBlock.NewGetElementPtr(elemType, ptrValue, indexValue)

		// Use volatile store if the variable is volatile (for MMIO)
		isVolatile := varInfo.IsVolatile || (varInfo.GeckoType != nil && varInfo.GeckoType.IsVolatile())
		impl.NewVolatileStore(info.LocalContext.MainBlock, newValue, elemPtr, isVolatile)
		return
	}

	// Generate the value to assign
	newValue := impl.ExpressionToLLIRValue(assignment.Value, scope, varInfo.GeckoType)
	if newValue == nil {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to evaluate assignment value", assignment.Pos)
		return
	}

	// Update the value in the values map
	varInfo.Value = newValue
}

// NewBreak handles break statements (stub - LLVM control flow needs proper basic blocks)
func (impl *LLVMBackendImplementation) NewBreak(scope *ast.Ast) {
	// TODO: Implement proper LLVM break with basic block jumps
}

// NewContinue handles continue statements (stub - LLVM control flow needs proper basic blocks)
func (impl *LLVMBackendImplementation) NewContinue(scope *ast.Ast) {
	// TODO: Implement proper LLVM continue with basic block jumps
}

// NewCImport handles cimport statements (no-op for LLVM - linking handled externally)
func (impl *LLVMBackendImplementation) NewCImport(scope *ast.Ast, cimport *tokens.CImport) {
	// LLVM backend handles C interop through extern declarations, not includes
	// The WithObject/WithLibrary info can be used by the linker stage
}

// IntrinsicStatement handles intrinsic calls as statements
func (impl *LLVMBackendImplementation) IntrinsicStatement(scope *ast.Ast, i *tokens.Intrinsic) {
	// TODO: Implement LLVM intrinsics
}
