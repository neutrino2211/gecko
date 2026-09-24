// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"github.com/neutrino2211/gecko/tokens"
)

// DetectFeatures analyzes a file and returns which features it uses
func DetectFeatures(file *tokens.File) []Feature {
	detector := &featureDetector{
		used: make(map[Feature]bool),
	}

	detector.analyzeFile(file)

	var features []Feature
	for f := range detector.used {
		features = append(features, f)
	}
	return features
}

type featureDetector struct {
	used map[Feature]bool
}

func (d *featureDetector) mark(f Feature) {
	d.used[f] = true
}

func (d *featureDetector) analyzeFile(file *tokens.File) {
	// Imports
	if len(file.Imports) > 0 {
		d.mark(FeatureImports)
	}

	for _, entry := range file.Entries {
		d.analyzeEntry(entry)
	}
}

func (d *featureDetector) analyzeEntry(entry *tokens.Entry) {
	if entry == nil {
		return
	}

	// Control flow
	if entry.If != nil {
		d.mark(FeatureControlFlow)
		d.analyzeIf(entry.If)
	}
	if entry.Loop != nil {
		d.mark(FeatureControlFlow)
		d.analyzeLoop(entry.Loop)
	}
	if entry.Break != nil || entry.Continue != nil {
		d.mark(FeatureControlFlow)
		d.mark(FeatureLoopControl)
	}

	// Functions
	if entry.Method != nil {
		d.mark(FeatureFunctions)
		d.analyzeMethod(entry.Method)
	}

	// Variables
	if entry.Field != nil {
		d.mark(FeatureVariables)
		d.analyzeField(entry.Field)
	}

	// Classes
	if entry.Class != nil {
		d.mark(FeatureClasses)
		d.analyzeClass(entry.Class)
	}

	// Traits
	if entry.Trait != nil {
		d.mark(FeatureTraits)
	}

	// Implementations
	if entry.Implementation != nil {
		d.mark(FeatureImpl)
	}

	// Declarations (external)
	if entry.Declaration != nil {
		d.mark(FeatureExternDecl)
		d.analyzeDeclaration(entry.Declaration)
	}
	if entry.Foreign != nil {
		d.mark(FeatureExternDecl)
	}

	// Inline assembly
	if entry.Asm != nil {
		d.mark(FeatureInlineAsm)
	}

	// Return with expression
	if entry.Return != nil {
		d.analyzeExpression(entry.Return)
	}

	// Function calls
	if entry.FuncCall != nil {
		d.analyzeFuncCall(entry.FuncCall)
	}

	// Assignment
	if entry.Assignment != nil {
		d.analyzeAssignment(entry.Assignment)
	}

	// Intrinsics
	if entry.Intrinsic != nil {
		d.analyzeIntrinsic(entry.Intrinsic)
	}

	if entry.UnsafeBlock != nil {
		for _, nested := range entry.UnsafeBlock.Body {
			d.analyzeEntry(nested)
		}
	}
}

func (d *featureDetector) analyzeMethod(method *tokens.Method) {
	// Check for generics
	if len(method.TypeParams) > 0 {
		d.mark(FeatureGenerics)
	}

	// Check for attributes
	for _, attr := range method.Attributes {
		switch attr.Name {
		case "naked":
			d.mark(FeatureNaked)
		case "noreturn":
			d.mark(FeatureNoReturn)
		case "section":
			d.mark(FeatureSection)
		}
	}

	// Analyze parameters
	for _, arg := range method.Arguments {
		if arg.Type != nil {
			d.analyzeTypeRef(arg.Type)
		}
	}

	// Analyze return type
	if method.Type != nil {
		d.analyzeTypeRef(method.Type)
	}

	// Analyze body
	for _, entry := range method.Value {
		d.analyzeEntry(entry)
	}
}

func (d *featureDetector) analyzeClass(class *tokens.Class) {
	// Check for generics
	if len(class.TypeParams) > 0 {
		d.mark(FeatureGenerics)
	}

	// Check for attributes
	for _, attr := range class.Attributes {
		if attr.Name == "packed" {
			d.mark(FeaturePacked)
		}
	}

	// Analyze fields and methods
	for _, field := range class.Fields {
		if field.Field != nil {
			d.analyzeField(field.Field)
		}
		if field.Method != nil {
			d.mark(FeatureFunctions)
			d.analyzeMethod(field.Method)
		}
	}
}

func (d *featureDetector) analyzeField(field *tokens.Field) {
	if field.Type == nil && field.Value != nil {
		d.mark(FeatureTypeInfer)
	}
	if field.Type != nil {
		d.analyzeTypeRef(field.Type)
	}
	if field.Value != nil {
		d.analyzeExpression(field.Value)
	}
}

func (d *featureDetector) analyzeTypeRef(t *tokens.TypeRef) {
	if t == nil {
		return
	}

	// Pointer types
	if t.Pointer {
		d.mark(FeaturePointers)
	}

	// Volatile
	if t.Volatile {
		d.mark(FeatureVolatile)
	}

	// Arrays
	if t.Array != nil || t.Size != nil {
		d.mark(FeatureArrays)
	}

	// Generic type arguments
	if len(t.TypeArgs) > 0 {
		d.mark(FeatureGenerics)
		for _, ta := range t.TypeArgs {
			d.analyzeTypeRef(ta)
		}
	}

	// Function types
	if t.FuncType != nil {
		d.mark(FeatureFunctions)
	}

	// String type
	if t.Type == "string" {
		d.mark(FeatureStrings)
	}

	// Nested types
	if t.Array != nil {
		d.analyzeTypeRef(t.Array)
	}
	if t.Size != nil {
		d.analyzeTypeRef(t.Size.Type)
	}
}

func (d *featureDetector) analyzeDeclaration(decl *tokens.Declaration) {
	if decl.Method != nil {
		d.analyzeMethod(decl.Method)
	}
	if decl.Field != nil {
		d.analyzeField(decl.Field)
	}
}

func (d *featureDetector) analyzeIf(ifStmt *tokens.If) {
	d.analyzeExpression(ifStmt.Expression)
	for _, entry := range ifStmt.Value {
		d.analyzeEntry(entry)
	}
	if ifStmt.ElseIf != nil {
		d.analyzeElseIf(ifStmt.ElseIf)
	}
	if ifStmt.Else != nil {
		for _, entry := range ifStmt.Else.Value {
			d.analyzeEntry(entry)
		}
	}
}

func (d *featureDetector) analyzeElseIf(elseIf *tokens.ElseIf) {
	d.analyzeExpression(elseIf.Expression)
	for _, entry := range elseIf.Value {
		d.analyzeEntry(entry)
	}
	if elseIf.ElseIf != nil {
		d.analyzeElseIf(elseIf.ElseIf)
	}
	if elseIf.Else != nil {
		for _, entry := range elseIf.Else.Value {
			d.analyzeEntry(entry)
		}
	}
}

func (d *featureDetector) analyzeLoop(loop *tokens.Loop) {
	if loop.WhileExpr != nil {
		d.analyzeExpression(loop.WhileExpr)
	}
	if loop.ForExpression != nil {
		d.analyzeExpression(loop.ForExpression)
	}
	for _, entry := range loop.Value {
		d.analyzeEntry(entry)
	}
}

func (d *featureDetector) analyzeAssignment(assign *tokens.Assignment) {
	if assign.Index != nil {
		d.mark(FeatureArrays)
		d.analyzeExpression(assign.Index)
	}
	d.analyzeExpression(assign.Value)
}
