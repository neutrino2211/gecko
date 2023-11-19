package tokens

import (
	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/codegen"
)

func (i *If) ToLLIRBlock(scope *ast.Ast) {
	var elseBlock *ir.Block = nil
	ifBlockScope := &ast.Ast{
		Scope:  "cond_" + i.GetID(),
		Parent: scope,
	}

	ifBlockScope.Init(scope.ErrorScope, scope.ExecutionContext)
	ifBlockScope.Config = scope.Config
	ifBlockScope.LocalContext = codegen.NewLocalContext(nil)
	ifBlockScope.LocalContext.MainBlock = scope.LocalContext.MainBlock.Parent.NewBlock("branch_" + i.GetID())

	ifBlockScope.LoadPrimitives()

	t := true
	i.Value = append(i.Value, &Entry{VoidReturn: &t})

	assignEntriesToAst(i.Value, ifBlockScope)

	condVal := i.Expression.ToLLIRValue(scope, &TypeRef{
		Type: "bool",
	})

	if i.Else != nil {
		elseBlock = i.Else.ToLLIRBlock(scope)
	} else if i.ElseIf != nil {
		elseBlock = i.ElseIf.ToLLIRBlock(scope)
	}

	repr.Println(elseBlock)

	br := scope.LocalContext.MainBlock.NewCondBr(condVal, ifBlockScope.LocalContext.MainBlock, elseBlock)
	repr.Println(br)
}

func (ei *ElseIf) ToLLIRBlock(scope *ast.Ast) *ir.Block {
	var elseBlock *ir.Block = nil
	elseIfBlockScope := &ast.Ast{
		Scope:  "cond_" + ei.GetID(),
		Parent: scope,
	}

	elseIfBlockScope.Init(scope.ErrorScope, scope.ExecutionContext)
	elseIfBlockScope.Config = scope.Config
	elseIfBlockScope.LocalContext = codegen.NewLocalContext(nil)
	elseIfBlockScope.LocalContext.MainBlock = scope.LocalContext.MainBlock.Parent.NewBlock("branch_" + ei.GetID())

	elseIfBlockScope.LoadPrimitives()

	t := true
	ei.Value = append(ei.Value, &Entry{VoidReturn: &t})

	assignEntriesToAst(ei.Value, elseIfBlockScope)

	condVal := ei.Expression.ToLLIRValue(scope, &TypeRef{
		Type: "bool",
	})

	if ei.Else != nil {
		elseBlock = ei.Else.ToLLIRBlock(scope)
	}

	scope.LocalContext.MainBlock.NewCondBr(condVal, elseIfBlockScope.LocalContext.MainBlock, elseBlock)

	return elseIfBlockScope.LocalContext.MainBlock
}

func (e *Else) ToLLIRBlock(scope *ast.Ast) *ir.Block {
	elseBlockScope := &ast.Ast{
		Scope:  "cond_" + e.GetID(),
		Parent: scope,
	}

	elseBlockScope.Init(scope.ErrorScope, scope.ExecutionContext)
	elseBlockScope.Config = scope.Config
	elseBlockScope.LocalContext = codegen.NewLocalContext(nil)
	elseBlockScope.LocalContext.MainBlock = scope.LocalContext.MainBlock.Parent.NewBlock("branch_" + e.GetID())

	elseBlockScope.LoadPrimitives()

	t := true
	e.Value = append(e.Value, &Entry{VoidReturn: &t})

	assignEntriesToAst(e.Value, elseBlockScope)

	return elseBlockScope.LocalContext.MainBlock
}
