package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

// AnalysisContext holds parsed files and resolved symbols for analysis
type AnalysisContext struct {
	MainFile     *tokens.File
	ImportedFiles map[string]*tokens.File
	RootScope    *ast.Ast
	FilePath     string
}

// NewAnalysisContext creates an analysis context for a file
func NewAnalysisContext(filePath string, content string) (*AnalysisContext, error) {
	file, err := parser.Parser.ParseString(filePath, content)
	if err != nil {
		return nil, err
	}
	file.ComputeRanges()
	file.Path = filePath
	file.Content = content

	ctx := &AnalysisContext{
		MainFile:      file,
		ImportedFiles: make(map[string]*tokens.File),
		FilePath:      filePath,
	}

	// Build AST scope for symbol resolution
	ctx.RootScope = &ast.Ast{
		Scope: file.PackageName,
	}
	errorScope := errors.NewErrorScope("analysis", file.PackageName, content)
	ctx.RootScope.Init(errorScope)

	// Resolve imports
	ctx.resolveImports()

	// Register symbols in scope
	ctx.registerSymbols()

	return ctx, nil
}

// resolveImports parses imported modules
func (ctx *AnalysisContext) resolveImports() {
	baseDir := filepath.Dir(ctx.FilePath)
	geckoHome := getGeckoHome()
	stdlibPath := filepath.Join(geckoHome, "stdlib")

	for _, entry := range ctx.MainFile.Entries {
		if entry.Import == nil {
			continue
		}

		moduleName := entry.Import.ModuleName()
		pathComponents := entry.Import.Path
		relativePath := filepath.Join(pathComponents...)

		var searchPaths []string
		if len(pathComponents) > 0 && pathComponents[0] == "std" {
			stdRelativePath := filepath.Join(pathComponents[1:]...)
			searchPaths = []string{stdlibPath}
			relativePath = stdRelativePath
		} else {
			searchPaths = []string{baseDir}
		}

		for _, searchPath := range searchPaths {
			candidates := []string{
				filepath.Join(searchPath, relativePath+".gecko"),
				filepath.Join(searchPath, relativePath, "mod.gecko"),
			}

			for _, candidate := range candidates {
				if moduleContent, err := os.ReadFile(candidate); err == nil {
					moduleFile, parseErr := parser.Parser.ParseString(candidate, string(moduleContent))
					if parseErr == nil {
						moduleFile.ComputeRanges()
						moduleFile.Path = candidate
						moduleFile.Content = string(moduleContent)
						ctx.ImportedFiles[moduleName] = moduleFile
					}
					break
				}
			}
		}
	}
}

// registerSymbols adds all symbols to the AST scope
func (ctx *AnalysisContext) registerSymbols() {
	ctx.registerFileSymbols(ctx.MainFile, ctx.RootScope)

	for moduleName, moduleFile := range ctx.ImportedFiles {
		moduleScope := &ast.Ast{
			Scope:  moduleName,
			Parent: ctx.RootScope,
		}
		moduleScope.Init(ctx.RootScope.ErrorScope)
		ctx.RootScope.Children[moduleName] = moduleScope
		ctx.registerFileSymbols(moduleFile, moduleScope)
	}
}

// registerFileSymbols registers symbols from a file into a scope
func (ctx *AnalysisContext) registerFileSymbols(file *tokens.File, scope *ast.Ast) {
	for _, entry := range file.Entries {
		if entry.Class != nil {
			classScope := &ast.Ast{
				Scope:        entry.Class.Name,
				Parent:       scope,
				Visibility:   entry.Class.Visibility,
				OriginModule: file.PackageName,
			}
			classScope.Init(scope.ErrorScope)

			// Register class fields
			for _, field := range entry.Class.Fields {
				if field.Field != nil {
					classScope.Variables[field.Field.Name] = ast.Variable{
						Name:      field.Field.Name,
						IsPointer: field.Field.Type != nil && field.Field.Type.Pointer,
						IsConst:   field.Field.Type != nil && field.Field.Type.Const,
						Parent:    classScope,
					}
				}
				if field.Method != nil {
					classScope.Methods[field.Method.Name] = &ast.Method{
						Name:       field.Method.Name,
						Visibility: field.Method.Visibility,
						Parent:     classScope,
						Type:       getReturnType(field.Method.Type),
					}
				}
			}

			scope.Classes[entry.Class.Name] = classScope
		}

		if entry.Trait != nil {
			methods := []*ast.Method{}
			for _, field := range entry.Trait.Fields {
				methods = append(methods, &ast.Method{
					Name:       field.Name,
					Visibility: "public",
					Type:       getReturnType(field.Type),
				})
			}
			scope.Traits[entry.Trait.Name] = &methods
		}

		if entry.Method != nil {
			scope.Methods[entry.Method.Name] = &ast.Method{
				Name:       entry.Method.Name,
				Visibility: entry.Method.Visibility,
				Parent:     scope,
				Type:       getReturnType(entry.Method.Type),
			}
		}

		if entry.Field != nil {
			scope.Variables[entry.Field.Name] = ast.Variable{
				Name:      entry.Field.Name,
				IsPointer: entry.Field.Type != nil && entry.Field.Type.Pointer,
				IsConst:   entry.Field.Mutability == "const",
				Parent:    scope,
			}
		}

		// Register implementation methods
		if entry.Implementation != nil && entry.Implementation.GetFor() != "" {
			if classScope, ok := scope.Classes[entry.Implementation.GetFor()]; ok {
				traitName := entry.Implementation.GetName()
				methods := []*ast.Method{}
				for _, field := range entry.Implementation.GetFields() {
					method := &ast.Method{
						Name:       field.Name,
						Visibility: "public",
						Parent:     classScope,
						Type:       getReturnType(field.Type),
					}
					methods = append(methods, method)
				}
				classScope.Traits[traitName] = &methods
			}
		}
	}
}

func getReturnType(t *tokens.TypeRef) string {
	if t == nil {
		return "void"
	}
	return FormatTypeRef(t)
}

// FormatTypeRef formats a TypeRef as a string (shared with LSP)
func FormatTypeRef(t *tokens.TypeRef) string {
	if t == nil {
		return "unknown"
	}

	var sb strings.Builder

	if t.Array != nil {
		sb.WriteString("[]")
		sb.WriteString(FormatTypeRef(t.Array))
		return sb.String()
	}

	if t.Size != nil {
		sb.WriteString(fmt.Sprintf("[%s]", t.Size.Size))
		sb.WriteString(FormatTypeRef(t.Size.Type))
		return sb.String()
	}

	if t.FuncType != nil {
		sb.WriteString("func(")
		for i, pt := range t.FuncType.ParamTypes {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(FormatTypeRef(pt))
		}
		sb.WriteString(")")
		if t.FuncType.ReturnType != nil {
			sb.WriteString(": ")
			sb.WriteString(FormatTypeRef(t.FuncType.ReturnType))
		}
		return sb.String()
	}

	if t.Module != "" {
		sb.WriteString(t.Module)
		sb.WriteString(".")
	}
	sb.WriteString(t.Type)

	if len(t.TypeArgs) > 0 {
		sb.WriteString("<")
		for i, ta := range t.TypeArgs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(FormatTypeRef(ta))
		}
		sb.WriteString(">")
	}

	if t.Volatile {
		sb.WriteString(" volatile")
	}

	if t.Pointer {
		sb.WriteString("*")
	}

	if t.NonNull {
		sb.WriteString("!")
	}

	return sb.String()
}

// FormatMethodSignature formats a method signature
func FormatMethodSignature(method *tokens.Method) string {
	var sb strings.Builder
	sb.WriteString("func ")
	sb.WriteString(method.Name)

	if len(method.TypeParams) > 0 {
		sb.WriteString("<")
		for i, tp := range method.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tp.Name)
			if tp.Trait != "" {
				sb.WriteString(" is ")
				sb.WriteString(tp.Trait)
			}
		}
		sb.WriteString(">")
	}

	sb.WriteString("(")
	for i, arg := range method.Arguments {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name)
		if arg.Type != nil {
			sb.WriteString(": ")
			sb.WriteString(FormatTypeRef(arg.Type))
		}
	}
	sb.WriteString(")")

	if method.Type != nil {
		sb.WriteString(": ")
		sb.WriteString(FormatTypeRef(method.Type))
	}

	return sb.String()
}

// FormatClassType formats a class declaration
func FormatClassType(class *tokens.Class) string {
	var sb strings.Builder
	if class.Visibility != "" {
		sb.WriteString(class.Visibility + " ")
	}
	sb.WriteString("class ")
	sb.WriteString(class.Name)
	if len(class.TypeParams) > 0 {
		sb.WriteString("<")
		for i, tp := range class.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tp.Name)
			if tp.Trait != "" {
				sb.WriteString(" is ")
				sb.WriteString(tp.Trait)
			}
		}
		sb.WriteString(">")
	}
	return sb.String()
}

// FormatTraitType formats a trait declaration
func FormatTraitType(trait *tokens.Trait) string {
	var sb strings.Builder
	if trait.Visibility != "" {
		sb.WriteString(trait.Visibility + " ")
	}
	sb.WriteString("trait ")
	sb.WriteString(trait.Name)
	if len(trait.TypeParams) > 0 {
		sb.WriteString("<")
		for i, tp := range trait.TypeParams {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tp.Name)
		}
		sb.WriteString(">")
	}
	return sb.String()
}

// IsPublic checks if a symbol is publicly accessible
func IsPublic(visibility string) bool {
	return visibility == "public" || visibility == "external"
}

// InferExpressionType infers the type of an expression
func InferExpressionType(expr *tokens.Expression, ctx *AnalysisContext) string {
	if expr == nil {
		return "unknown"
	}

	// Use the tokens package's inference first
	resolveSymbol := func(name string) *tokens.TypeRef {
		// Look up in context
		if varInfo := ctx.findVariable(name); varInfo != nil {
			return varInfo.Type
		}
		return nil
	}

	if inferred := tokens.InferType(expr, resolveSymbol); inferred != nil {
		return FormatTypeRef(inferred)
	}

	// Fall back to basic inference
	primary := getPrimaryFromExpr(expr)
	if primary == nil || primary.Literal == nil {
		return "unknown"
	}

	lit := primary.Literal

	if lit.StructType != "" {
		return lit.StructType
	}
	if lit.Number != "" {
		return "int"
	}
	if lit.String != "" {
		return "string"
	}
	if lit.Bool != "" {
		return "bool"
	}

	if lit.FuncCall != nil {
		return ctx.inferFuncCallType(lit.FuncCall)
	}

	if lit.Symbol != "" {
		if varInfo := ctx.findVariable(lit.Symbol); varInfo != nil && varInfo.Type != nil {
			return FormatTypeRef(varInfo.Type)
		}
	}

	return "unknown"
}

// VariableInfo holds information about a variable
type VariableInfo struct {
	Name       string
	Type       *tokens.TypeRef
	Mutability string
	Line       int
}

// findVariable looks up a variable in the analysis context
func (ctx *AnalysisContext) findVariable(name string) *VariableInfo {
	// Search in main file
	for _, entry := range ctx.MainFile.Entries {
		if entry.Method != nil {
			// Check arguments
			for _, arg := range entry.Method.Arguments {
				if arg.Name == name {
					return &VariableInfo{Name: name, Type: arg.Type, Line: arg.Pos.Line}
				}
			}
			// Check local variables
			if varInfo := findVarInEntries(entry.Method.Value, name); varInfo != nil {
				return varInfo
			}
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					for _, arg := range field.Method.Arguments {
						if arg.Name == name {
							return &VariableInfo{Name: name, Type: arg.Type, Line: arg.Pos.Line}
						}
					}
					if varInfo := findVarInEntries(field.Method.Value, name); varInfo != nil {
						return varInfo
					}
				}
			}
		}
		if entry.Field != nil && entry.Field.Name == name {
			return &VariableInfo{
				Name:       name,
				Type:       entry.Field.Type,
				Mutability: entry.Field.Mutability,
				Line:       entry.Field.Pos.Line,
			}
		}
	}
	return nil
}

func findVarInEntries(entries []*tokens.Entry, name string) *VariableInfo {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == name {
			return &VariableInfo{
				Name:       name,
				Type:       entry.Field.Type,
				Mutability: entry.Field.Mutability,
				Line:       entry.Field.Pos.Line,
			}
		}
		if entry.If != nil {
			if v := findVarInEntries(entry.If.Value, name); v != nil {
				return v
			}
		}
		if entry.Loop != nil {
			if v := findVarInEntries(entry.Loop.Value, name); v != nil {
				return v
			}
		}
	}
	return nil
}

func (ctx *AnalysisContext) inferFuncCallType(f *tokens.FuncCall) string {
	if f == nil {
		return "unknown"
	}

	// Static method call
	if f.StaticType != "" {
		if classScope, ok := ctx.RootScope.Classes[f.StaticType]; ok {
			if method, ok := classScope.Methods[f.Function]; ok {
				return method.Type
			}
		}
		return f.StaticType
	}

	// Regular function call
	if method, ok := ctx.RootScope.Methods[f.Function]; ok {
		return method.Type
	}

	return fmt.Sprintf("(result of %s())", f.Function)
}

func getPrimaryFromExpr(expr *tokens.Expression) *tokens.Primary {
	if expr.LogicalOr == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary == nil {
		return nil
	}
	return expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary
}

// getGeckoHome returns the Gecko home directory (same logic as compiler)
func getGeckoHome() string {
	hasStdlib := func(path string) bool {
		_, err := os.Stat(filepath.Join(path, "stdlib"))
		return err == nil
	}

	if home := os.Getenv("GECKO_HOME"); home != "" && hasStdlib(home) {
		return home
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		if hasStdlib("/usr/local/lib/gecko") {
			return "/usr/local/lib/gecko"
		}
		if home := os.Getenv("HOME"); home != "" {
			userPath := filepath.Join(home, ".gecko")
			if hasStdlib(userPath) {
				return userPath
			}
		}
	case "windows":
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			winPath := filepath.Join(appData, "gecko")
			if hasStdlib(winPath) {
				return winPath
			}
		}
	}

	if wd, err := os.Getwd(); err == nil && hasStdlib(wd) {
		return wd
	}
	return "."
}
