package docgen

import (
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

// ExtractPackageDoc extracts documentation from a parsed file
func ExtractPackageDoc(file *tokens.File, sourcePath string) PackageDoc {
	pkg := PackageDoc{
		Name:       file.PackageName,
		SourceFile: sourcePath,
		Classes:    []DocItem{},
		Traits:     []DocItem{},
		Functions:  []DocItem{},
		Fields:     []DocItem{},
	}

	for _, entry := range file.Entries {
		if entry.Class != nil {
			pkg.Classes = append(pkg.Classes, ExtractClassDoc(entry.Class, sourcePath))
		} else if entry.Trait != nil {
			pkg.Traits = append(pkg.Traits, ExtractTraitDoc(entry.Trait, sourcePath))
		} else if entry.Method != nil {
			pkg.Functions = append(pkg.Functions, ExtractMethodDoc(entry.Method, sourcePath))
		} else if entry.Field != nil {
			pkg.Fields = append(pkg.Fields, ExtractFieldDoc(entry.Field, sourcePath))
		}
	}

	return pkg
}

// ExtractClassDoc extracts documentation from a class
func ExtractClassDoc(class *tokens.Class, sourcePath string) DocItem {
	item := DocItem{
		Name:       class.Name,
		Kind:       "class",
		DocComment: joinDocComment(class.DocComment),
		Signature:  buildClassSignature(class),
		Visibility: normalizeVisibility(class.Visibility),
		SourceFile: sourcePath,
		Line:       class.Pos.Line,
		TypeParams: extractTypeParams(class.TypeParams),
		Fields:     []DocItem{},
		Methods:    []DocItem{},
	}

	for _, field := range class.Fields {
		if field.Field != nil {
			item.Fields = append(item.Fields, ExtractFieldDoc(field.Field, sourcePath))
		} else if field.Method != nil {
			item.Methods = append(item.Methods, ExtractMethodDoc(field.Method, sourcePath))
		}
	}

	return item
}

// ExtractTraitDoc extracts documentation from a trait
func ExtractTraitDoc(trait *tokens.Trait, sourcePath string) DocItem {
	item := DocItem{
		Name:       trait.Name,
		Kind:       "trait",
		DocComment: joinDocComment(trait.DocComment),
		Signature:  buildTraitSignature(trait),
		Visibility: "public",
		SourceFile: sourcePath,
		Line:       trait.Pos.Line,
		Methods:    []DocItem{},
	}

	for _, field := range trait.Fields {
		item.Methods = append(item.Methods, ExtractTraitMethodDoc(field, sourcePath))
	}

	return item
}

// ExtractMethodDoc extracts documentation from a method
func ExtractMethodDoc(method *tokens.Method, sourcePath string) DocItem {
	item := DocItem{
		Name:       method.Name,
		Kind:       "method",
		DocComment: joinDocComment(method.DocComment),
		Signature:  buildMethodSignature(method),
		Visibility: normalizeVisibility(method.Visibility),
		SourceFile: sourcePath,
		Line:       method.Pos.Line,
		TypeParams: extractTypeParams(method.TypeParams),
		Arguments:  extractArguments(method.Arguments),
		ReturnType: typeRefToString(method.Type),
	}

	return item
}

// ExtractTraitMethodDoc extracts documentation from a trait method
func ExtractTraitMethodDoc(field *tokens.ImplementationField, sourcePath string) DocItem {
	return DocItem{
		Name:       field.Name,
		Kind:       "method",
		DocComment: "", // ImplementationField doesn't have DocComment yet
		Signature:  buildImplFieldSignature(field),
		Visibility: "public",
		SourceFile: sourcePath,
		Line:       field.Pos.Line,
		Arguments:  extractArguments(field.Arguments),
		ReturnType: typeRefToString(field.Type),
	}
}

// ExtractFieldDoc extracts documentation from a field
func ExtractFieldDoc(field *tokens.Field, sourcePath string) DocItem {
	return DocItem{
		Name:       field.Name,
		Kind:       "field",
		DocComment: joinDocComment(field.DocComment),
		Signature:  buildFieldSignature(field),
		Visibility: normalizeVisibility(field.Visibility),
		SourceFile: sourcePath,
		Line:       field.Pos.Line,
		FieldType:  typeRefToString(field.Type),
		IsMutable:  field.Mutability == "let",
	}
}

// Helper functions

func joinDocComment(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	// Strip leading "///" and whitespace from each line
	cleaned := make([]string, len(lines))
	for i, line := range lines {
		// Remove "///" prefix
		line = strings.TrimPrefix(line, "///")
		// Remove leading space (common convention)
		line = strings.TrimPrefix(line, " ")
		cleaned[i] = line
	}

	return strings.Join(cleaned, "\n")
}

func normalizeVisibility(vis string) string {
	if vis == "" {
		return "public"
	}
	return vis
}

func extractTypeParams(params []*tokens.TypeParam) []TypeParamDoc {
	result := make([]TypeParamDoc, len(params))
	for i, param := range params {
		result[i] = TypeParamDoc{
			Name:       param.Name,
			Constraint: param.Trait,
		}
	}
	return result
}

func extractArguments(args []*tokens.Value) []ArgDoc {
	result := make([]ArgDoc, len(args))
	for i, arg := range args {
		result[i] = ArgDoc{
			Name: arg.Name,
			Type: typeRefToString(arg.Type),
		}
	}
	return result
}

func typeRefToString(t *tokens.TypeRef) string {
	if t == nil {
		return "void"
	}

	var result string

	if t.Array != nil {
		result = "[" + typeRefToString(t.Array) + "]"
	} else if t.FuncType != nil {
		params := make([]string, len(t.FuncType.ParamTypes))
		for i, p := range t.FuncType.ParamTypes {
			params[i] = typeRefToString(p)
		}
		result = "func(" + strings.Join(params, ", ") + ")"
		if t.FuncType.ReturnType != nil {
			result += ": " + typeRefToString(t.FuncType.ReturnType)
		}
	} else {
		result = t.Type
		if len(t.TypeArgs) > 0 {
			args := make([]string, len(t.TypeArgs))
			for i, arg := range t.TypeArgs {
				args[i] = typeRefToString(arg)
			}
			result += "<" + strings.Join(args, ", ") + ">"
		}
	}

	if t.Volatile {
		result += " volatile"
	}

	if t.Pointer {
		result += "*"
	}

	return result
}

func buildClassSignature(class *tokens.Class) string {
	sig := "class " + class.Name

	if len(class.TypeParams) > 0 {
		params := make([]string, len(class.TypeParams))
		for i, p := range class.TypeParams {
			if p.Trait != "" {
				params[i] = p.Name + " is " + p.Trait
			} else {
				params[i] = p.Name
			}
		}
		sig += "<" + strings.Join(params, ", ") + ">"
	}

	return sig
}

func buildTraitSignature(trait *tokens.Trait) string {
	sig := "trait " + trait.Name

	if len(trait.TypeParams) > 0 {
		params := make([]string, len(trait.TypeParams))
		for i, p := range trait.TypeParams {
			if p.Trait != "" {
				params[i] = p.Name + " is " + p.Trait
			} else {
				params[i] = p.Name
			}
		}
		sig += "<" + strings.Join(params, ", ") + ">"
	}

	return sig
}

func buildMethodSignature(method *tokens.Method) string {
	sig := "func " + method.Name

	if len(method.TypeParams) > 0 {
		params := make([]string, len(method.TypeParams))
		for i, p := range method.TypeParams {
			if p.Trait != "" {
				params[i] = p.Name + " is " + p.Trait
			} else {
				params[i] = p.Name
			}
		}
		sig += "<" + strings.Join(params, ", ") + ">"
	}

	sig += "("
	args := make([]string, len(method.Arguments))
	for i, arg := range method.Arguments {
		args[i] = arg.Name + ": " + typeRefToString(arg.Type)
	}
	sig += strings.Join(args, ", ") + ")"

	if method.Type != nil {
		sig += ": " + typeRefToString(method.Type)
	}

	return sig
}

func buildFieldSignature(field *tokens.Field) string {
	sig := field.Mutability + " " + field.Name

	if field.Type != nil {
		sig += ": " + typeRefToString(field.Type)
	}

	return sig
}

func buildImplFieldSignature(field *tokens.ImplementationField) string {
	sig := "func " + field.Name + "("

	args := make([]string, len(field.Arguments))
	for i, arg := range field.Arguments {
		args[i] = arg.Name + ": " + typeRefToString(arg.Type)
	}
	sig += strings.Join(args, ", ") + ")"

	if field.Type != nil {
		sig += ": " + typeRefToString(field.Type)
	}

	return sig
}
