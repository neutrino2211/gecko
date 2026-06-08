// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetSignatureHelp returns signature help for a function call at the given position
func GetSignatureHelp(content, filePath string, line, col int) *protocol.SignatureHelp {
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return nil
	}
	lineText := lines[line]
	if col > len(lineText) {
		col = len(lineText)
	}

	// Find the opening parenthesis and count commas to determine active parameter
	parenDepth := 0
	activeParam := 0
	callStart := -1

	// Scan backwards from cursor to find the function call
	for i := col - 1; i >= 0; i-- {
		ch := lineText[i]
		if ch == ')' {
			parenDepth++
		} else if ch == '(' {
			if parenDepth == 0 {
				callStart = i
				break
			}
			parenDepth--
		} else if ch == ',' && parenDepth == 0 {
			activeParam++
		}
	}

	if callStart < 0 {
		return nil
	}

	// Extract function/method name before the parenthesis
	nameEnd := callStart
	nameStart := nameEnd
	for nameStart > 0 && isIdentChar(lineText[nameStart-1]) {
		nameStart--
	}
	if nameStart >= nameEnd {
		return nil
	}
	funcName := lineText[nameStart:nameEnd]

	// Check if it's a method call (preceded by '.')
	var objName string
	if nameStart > 0 && lineText[nameStart-1] == '.' {
		objEnd := nameStart - 1
		objStart := objEnd
		for objStart > 0 && isIdentChar(lineText[objStart-1]) {
			objStart--
		}
		if objStart < objEnd {
			objName = lineText[objStart:objEnd]
		}
	}

	// Check if it's a static method call (preceded by '::')
	var typeName string
	if nameStart > 1 && lineText[nameStart-1] == ':' && lineText[nameStart-2] == ':' {
		typeEnd := nameStart - 2
		typeStart := typeEnd
		for typeStart > 0 && isIdentChar(lineText[typeStart-1]) {
			typeStart--
		}
		if typeStart < typeEnd {
			typeName = lineText[typeStart:typeEnd]
		}
	}

	// Parse the file to look up function signatures
	sanitizedContent := sanitizeForParsing(content, line)
	file, _ := parser.Parser.ParseString(filePath, sanitizedContent)
	if file == nil {
		return nil
	}
	file.ComputeRanges()

	var signature *protocol.SignatureInformation

	if typeName != "" {
		// Static method call - look up in impl blocks
		signature = findStaticMethodSignature(file, typeName, funcName)
	} else if objName != "" {
		// Instance method call - resolve variable type and look up method
		varType := lookupVariableTypeInScope(file, objName, line+1)
		if varType == "" {
			varType = lookupVariableType(file, objName)
		}
		if varType != "" {
			parsedType := parseGenericType(varType)
			signature = findMethodSignature(file, filePath, parsedType.BaseName, funcName, parsedType.TypeArgs)
		}
	} else {
		// Free function call
		signature = findFunctionSignature(file, funcName)
	}

	if signature == nil {
		return nil
	}

	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{*signature},
		ActiveSignature: 0,
		ActiveParameter: uint32(activeParam),
	}
}

// findFunctionSignature finds a top-level function signature
func findFunctionSignature(file *tokens.File, funcName string) *protocol.SignatureInformation {
	for _, entry := range file.Entries {
		if entry.Method != nil && entry.Method.Name == funcName {
			return buildSignatureInfo(entry.Method.Name, entry.Method.Arguments, entry.Method.Type)
		}
	}
	return nil
}

// findStaticMethodSignature finds a static method signature in impl blocks
func findStaticMethodSignature(file *tokens.File, typeName, methodName string) *protocol.SignatureInformation {
	for _, entry := range file.Entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, typeName) {
			for _, field := range entry.Implementation.GetFields() {
				if field.Name == methodName {
					return buildImplSignatureInfo(field)
				}
			}
		}
	}
	return nil
}

// findMethodSignature finds an instance method signature
func findMethodSignature(file *tokens.File, filePath, typeName, methodName string, typeArgs []string) *protocol.SignatureInformation {
	// Get type parameters for substitution
	var typeParams []string
	for _, entry := range file.Entries {
		if entry.Class != nil && entry.Class.Name == typeName {
			for _, tp := range entry.Class.TypeParams {
				typeParams = append(typeParams, tp.Name)
			}
			break
		}
	}

	// Look in impl blocks
	for _, entry := range file.Entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, typeName) {
			for _, field := range entry.Implementation.GetFields() {
				if field.Name == methodName {
					sig := buildImplSignatureInfo(field)
					if len(typeArgs) > 0 {
						sig.Label = substituteTypeParams(sig.Label, typeParams, typeArgs)
					}
					return sig
				}
			}
		}
	}
	return nil
}

// buildSignatureInfo builds a SignatureInformation from function arguments
func buildSignatureInfo(name string, args []*tokens.Value, returnType *tokens.TypeRef) *protocol.SignatureInformation {
	var params []protocol.ParameterInformation
	var paramStrs []string

	for _, arg := range args {
		typeStr := "unknown"
		if arg.Type != nil {
			typeStr = analysis.FormatTypeRef(arg.Type)
		}
		paramStr := fmt.Sprintf("%s: %s", arg.Name, typeStr)
		paramStrs = append(paramStrs, paramStr)
		params = append(params, protocol.ParameterInformation{
			Label: paramStr,
		})
	}

	retStr := "void"
	if returnType != nil {
		retStr = analysis.FormatTypeRef(returnType)
	}

	label := fmt.Sprintf("%s(%s): %s", name, strings.Join(paramStrs, ", "), retStr)

	return &protocol.SignatureInformation{
		Label:      label,
		Parameters: params,
	}
}

// buildImplSignatureInfo builds a SignatureInformation from an impl field
func buildImplSignatureInfo(field *tokens.ImplementationField) *protocol.SignatureInformation {
	var params []protocol.ParameterInformation
	var paramStrs []string

	for _, arg := range field.Arguments {
		typeStr := "unknown"
		if arg.Type != nil {
			typeStr = analysis.FormatTypeRef(arg.Type)
		}
		paramStr := fmt.Sprintf("%s: %s", arg.Name, typeStr)
		paramStrs = append(paramStrs, paramStr)
		params = append(params, protocol.ParameterInformation{
			Label: paramStr,
		})
	}

	retStr := "void"
	if field.Type != nil {
		retStr = analysis.FormatTypeRef(field.Type)
	}

	label := fmt.Sprintf("%s(%s): %s", field.Name, strings.Join(paramStrs, ", "), retStr)

	return &protocol.SignatureInformation{
		Label:      label,
		Parameters: params,
	}
}
