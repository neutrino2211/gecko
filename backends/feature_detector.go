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

func (d *featureDetector) analyzeExpression(expr *tokens.Expression) {
	if expr == nil || expr.GetLogicalOr() == nil {
		return
	}
	d.analyzeLogicalOr(expr.GetLogicalOr())
}

func (d *featureDetector) analyzeLogicalOr(lo *tokens.LogicalOr) {
	if lo == nil {
		return
	}
	if lo.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeLogicalAnd(lo.LogicalAnd)
	if lo.Next != nil {
		d.analyzeLogicalOr(lo.Next)
	}
}

func (d *featureDetector) analyzeLogicalAnd(la *tokens.LogicalAnd) {
	if la == nil {
		return
	}
	if la.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeEquality(la.Equality)
	if la.Next != nil {
		d.analyzeLogicalAnd(la.Next)
	}
}

func (d *featureDetector) analyzeEquality(eq *tokens.Equality) {
	if eq == nil {
		return
	}
	if eq.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeComparison(eq.Comparison)
	if eq.Next != nil {
		d.analyzeEquality(eq.Next)
	}
}

func (d *featureDetector) analyzeComparison(cmp *tokens.Comparison) {
	if cmp == nil {
		return
	}
	if cmp.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeAddition(cmp.Addition)
	if cmp.Next != nil {
		d.analyzeComparison(cmp.Next)
	}
}

func (d *featureDetector) analyzeAddition(add *tokens.Addition) {
	if add == nil {
		return
	}
	if add.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeMultiplication(add.Multiplication)
	if add.Next != nil {
		d.analyzeAddition(add.Next)
	}
}

func (d *featureDetector) analyzeMultiplication(mul *tokens.Multiplication) {
	if mul == nil {
		return
	}
	if mul.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeUnary(mul.Unary)
	if mul.Next != nil {
		d.analyzeMultiplication(mul.Next)
	}
}

func (d *featureDetector) analyzeUnary(un *tokens.Unary) {
	if un == nil {
		return
	}
	if un.Op != "" {
		d.mark(FeatureBasicOps)
	}
	if un.Cast != nil {
		d.mark(FeatureCasts)
		d.analyzeTypeRef(un.Cast.Type)
	}
	if un.Primary != nil {
		d.analyzePrimary(un.Primary)
	}
	if un.Unary != nil {
		d.analyzeUnary(un.Unary)
	}
}

func (d *featureDetector) analyzePrimary(prim *tokens.Primary) {
	if prim == nil {
		return
	}
	if prim.SubExpression != nil {
		d.analyzeExpression(prim.SubExpression)
	}
	if prim.Literal != nil {
		d.analyzeLiteral(prim.Literal)
	}
}

func (d *featureDetector) analyzeLiteral(lit *tokens.Literal) {
	if lit == nil {
		return
	}

	// Address-of operator
	if lit.IsPointer {
		d.mark(FeatureAddressOf)
		d.mark(FeaturePointers)
	}

	// String literals
	if lit.String != "" {
		d.mark(FeatureStrings)
	}

	// Struct literals
	if lit.StructType != "" {
		d.mark(FeatureStructs)
	}

	// Array literals
	if len(lit.Array) > 0 {
		d.mark(FeatureArrays)
	}

	// Function calls
	if lit.FuncCall != nil {
		d.analyzeFuncCall(lit.FuncCall)
	}

	// Intrinsics
	if lit.Intrinsic != nil {
		d.analyzeIntrinsic(lit.Intrinsic)
	}

	// Method chains
	for _, chain := range lit.Chain {
		if chain.IsMethodCall() {
			d.mark(FeatureFunctions)
		}
	}

	// Array indexing
	if lit.ArrayIndex != nil {
		d.mark(FeatureArrays)
		d.analyzeExpression(lit.ArrayIndex)
	}
}

func (d *featureDetector) analyzeFuncCall(fc *tokens.FuncCall) {
	if fc == nil {
		return
	}
	d.mark(FeatureFunctions)

	// Generic type args
	if len(fc.TypeArgs) > 0 || len(fc.StaticTypeArgs) > 0 {
		d.mark(FeatureGenerics)
	}
}

func (d *featureDetector) analyzeIntrinsic(intr *tokens.Intrinsic) {
	if intr == nil {
		return
	}

	switch intr.Name {
	case "deref":
		d.mark(FeatureDeref)
		d.mark(FeaturePointers)
	case "write_volatile":
		d.mark(FeatureVolatile)
		d.mark(FeaturePointers)
	case "is_null":
		d.mark(FeaturePointers)
	case "size_of", "align_of":
		// These are core features
	}

	for _, arg := range intr.Args {
		d.analyzeExpression(arg)
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
