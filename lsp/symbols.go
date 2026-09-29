// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import "strings"

// ParsedGenericType represents a parsed generic type like Vec<int>
type ParsedGenericType struct {
	BaseName string   // The base type name (e.g., "Vec")
	TypeArgs []string // The type arguments (e.g., ["int"])
}

// parseGenericType parses a type string like "Vec<int>" into its components
func parseGenericType(typeStr string) ParsedGenericType {
	typeStr = strings.TrimRight(typeStr, "*!")

	// Find the generic brackets
	ltIdx := strings.Index(typeStr, "<")
	if ltIdx == -1 {
		return ParsedGenericType{BaseName: typeStr}
	}

	baseName := typeStr[:ltIdx]

	gtIdx := strings.LastIndex(typeStr, ">")
	if gtIdx == -1 || gtIdx <= ltIdx {
		return ParsedGenericType{BaseName: baseName}
	}

	argsStr := typeStr[ltIdx+1 : gtIdx]
	var args []string
	depth := 0
	start := 0
	for i, ch := range argsStr {
		switch ch {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(argsStr[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(argsStr[start:]))

	return ParsedGenericType{BaseName: baseName, TypeArgs: args}
}

// substituteTypeParams replaces type parameters with actual type arguments in a type string
func substituteTypeParams(typeStr string, typeParams []string, typeArgs []string) string {
	if len(typeParams) == 0 || len(typeArgs) == 0 {
		return typeStr
	}

	result := typeStr
	for i, param := range typeParams {
		if i < len(typeArgs) {
			// Replace whole word occurrences only
			result = replaceTypeParam(result, param, typeArgs[i])
		}
	}
	return result
}

// replaceTypeParam replaces a type parameter with its argument, handling word boundaries
func replaceTypeParam(str, param, replacement string) string {
	// Simple word boundary replacement
	var result strings.Builder
	i := 0
	for i < len(str) {
		// Check if we're at the start of the parameter
		if strings.HasPrefix(str[i:], param) {
			// Check word boundaries
			before := i == 0 || !isIdentChar(str[i-1])
			after := i+len(param) >= len(str) || !isIdentChar(str[i+len(param)])
			if before && after {
				result.WriteString(replacement)
				i += len(param)
				continue
			}
		}
		result.WriteByte(str[i])
		i++
	}
	return result.String()
}
