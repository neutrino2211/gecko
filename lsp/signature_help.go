// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetSignatureHelp returns signature help for a function call at the given position.
// When ctx is non-nil it resolves signatures from the shared semantic graph so the
// displayed signature matches the compiler (after generic substitution and trait
// resolution), falling back to file-based lookup otherwise.
func GetSignatureHelp(ctx *analysis.AnalysisContext, content, filePath string, line, col int) *protocol.SignatureHelp {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return nil
	}
	lineText := lines[line]
	if col < 0 {
		return nil
	}
	if col > len(lineText) {
		col = len(lineText)
	}
	offset := col
	for i := 0; i < line; i++ {
		offset += len(lines[i]) + 1
	}
	callStart, activeParam := openCallAt(content[:offset])
	if callStart < 0 {
		return nil
	}

	callText := content[:offset]
	nameEnd := callStart
	for nameEnd > 0 && (callText[nameEnd-1] == ' ' || callText[nameEnd-1] == '\t') {
		nameEnd--
	}
	nameStart := nameEnd
	for nameStart > 0 && isIdentChar(callText[nameStart-1]) {
		nameStart--
	}
	if nameStart >= nameEnd {
		return nil
	}
	funcName := callText[nameStart:nameEnd]

	// Check if it's a method call (preceded by '.')
	var objName string
	if nameStart > 0 && callText[nameStart-1] == '.' {
		objEnd := nameStart - 1
		objStart := objEnd
		for objStart > 0 && isIdentChar(callText[objStart-1]) {
			objStart--
		}
		if objStart < objEnd {
			objName = callText[objStart:objEnd]
		}
	}

	// Check if it's a static method call (preceded by '::')
	var typeName string
	if nameStart > 1 && callText[nameStart-1] == ':' && callText[nameStart-2] == ':' {
		typeEnd := nameStart - 2
		typeStart := typeEnd
		for typeStart > 0 && isIdentChar(callText[typeStart-1]) {
			typeStart--
		}
		if typeStart < typeEnd {
			typeName = callText[typeStart:typeEnd]
		}
	}

	// Parse the file to look up function signatures (used as fallback)
	var file *tokens.File
	if ctx != nil {
		file = ctx.MainFile
	} else {
		sanitizedContent := sanitizeForParsing(content, line)
		file, _ = parser.Parser.ParseString(filePath, sanitizedContent)
	}
	if file == nil {
		return nil
	}
	file.ComputeRanges()

	var signature *protocol.SignatureInformation

	if ctx != nil && ctx.SemanticGraph != nil {
		signature = signatureFromGraph(ctx, typeName, objName, funcName, line+1, col)
	}

	if signature == nil {
		if typeName != "" {
			// Static method call - look up in impl blocks
			signature = findStaticMethodSignature(file, typeName, funcName)
		} else if objName != "" {
			// Instance method call - resolve variable type and look up method
			varType := receiverVariableType(ctx, file, objName, line+1)
			if varType != "" {
				parsedType := parseGenericType(varType)
				signature = findMethodSignature(file, filePath, parsedType.BaseName, funcName, parsedType.TypeArgs)
			}
		} else {
			// Free function call
			signature = findFunctionSignature(file, funcName)
		}
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

func openCallAt(content string) (int, int) {
	type frame struct {
		kind   byte
		start  int
		commas int
	}
	var stack []frame
	var quote byte
	lineComment := false
	blockComment := false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if lineComment {
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if ch == '*' && i+1 < len(content) && content[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if ch == '\\' {
				i++
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '/' && i+1 < len(content) {
			if content[i+1] == '/' {
				lineComment = true
				i++
				continue
			}
			if content[i+1] == '*' {
				blockComment = true
				i++
				continue
			}
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '(', '[', '{':
			stack = append(stack, frame{kind: ch, start: i})
		case ')', ']', '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ',':
			if len(stack) > 0 && stack[len(stack)-1].kind == '(' {
				stack[len(stack)-1].commas++
			}
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].kind == '(' {
			return stack[i].start, stack[i].commas
		}
	}
	return -1, 0
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

// signatureFromGraph resolves a call signature from the shared semantic graph,
// which is the compiler's authoritative view (generic substitution and trait
// resolution already applied).
func signatureFromGraph(ctx *analysis.AnalysisContext, typeName, objName, funcName string, line, col int) *protocol.SignatureInformation {
	sg := ctx.SemanticGraph
	if sg == nil {
		return nil
	}

	if typeName != "" {
		// Static method / enum constructor: Type::func
		for _, sig := range sg.MethodsOfType(typeName) {
			if sig.Name == funcName {
				return buildSignatureInfoFromSig(sig)
			}
		}
		return nil
	}

	if objName != "" {
		// Instance method: resolve the receiver type from the graph.
		t := ctx.VariableType(objName, line, col)
		if t != nil {
			for _, sig := range sg.MethodsForType(t) {
				if sig.Name == funcName {
					return buildSignatureInfoFromSig(sig)
				}
			}
		}
		return nil
	}

	// Free function call: search every module's functions by name.
	for _, sig := range sg.FunctionsNamed(funcName) {
		return buildSignatureInfoFromSig(sig)
	}
	return nil
}

// buildSignatureInfoFromSig builds a SignatureInformation from a resolved signature.
func buildSignatureInfoFromSig(sig *semantic.FunctionSignature) *protocol.SignatureInformation {
	var params []protocol.ParameterInformation
	var paramStrs []string

	for _, arg := range sig.Params {
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
	if sig.ReturnType != nil {
		retStr = analysis.FormatTypeRef(sig.ReturnType)
	}

	label := fmt.Sprintf("%s(%s): %s", sig.Name, strings.Join(paramStrs, ", "), retStr)

	return &protocol.SignatureInformation{
		Label:      label,
		Parameters: params,
	}
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
