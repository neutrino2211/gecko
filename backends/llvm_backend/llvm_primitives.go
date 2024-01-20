package llvmbackend

import (
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
)

type PrimitiveType struct {
	Class *ast.Ast
	Type  types.Type
}

// Special

var VoidType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "void",
	},
	Type: types.Void,
}

var UnknownType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "unknown",
	},
	Type: types.Void,
}

var BoolType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "bool",
	},
	Type: types.I1,
}

// String

var RawStringType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "string",
	},
	Type: types.NewPointer(types.I8),
}

// Ints

var IntType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "int",
	},
	Type: types.I64,
}

var Int8Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "int8",
	},
	Type: types.I8,
}

var Int16Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "int16",
	},
	Type: types.I16,
}

var Int32Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "int32",
	},
	Type: types.I32,
}

var Int64Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "int64",
	},
	Type: types.I64,
}

// Uints

var UintType = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "uint",
	},
	Type: types.I64,
}

var Uint8Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "uint8",
	},
	Type: types.I8,
}

var Uint16Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "uint16",
	},
	Type: types.I16,
}

var Uint32Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "uint32",
	},
	Type: types.I32,
}

var Uint64Type = &PrimitiveType{
	Class: &ast.Ast{
		Scope: "uint64",
	},
	Type: types.I64,
}

var Primitives = []*PrimitiveType{
	VoidType,
	IntType,
	RawStringType,
	BoolType,
}
