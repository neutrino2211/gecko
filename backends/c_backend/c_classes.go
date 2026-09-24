// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewClass handles class definitions
func (impls *CBackendImplementation) NewClass(scope *ast.Ast, c *tokens.Class) {
	classAst := &ast.Ast{
		Scope:        c.Name,
		Parent:       scope,
		Visibility:   c.Visibility,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	// Register fields in the class AST for type checking (needed for both generic and non-generic)
	for _, f := range c.Fields {
		if f.Field != nil {
			fieldVariable := ast.Variable{
				Name:      f.Field.Name,
				IsPointer: f.Field.Type != nil && f.Field.Type.Pointer,
				IsConst:   bindingIsConst(f.Field),
				Parent:    classAst,
			}
			classAst.Variables[f.Field.Name] = fieldVariable

			// Store type info for type checking (use generic param name as-is for generics)
			(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
				CType:     "", // Not resolved yet for generics
				GeckoType: f.Field.Type,
			}
		}
	}

	// Register methods in class AST for type checking (needed for both generic and non-generic)
	for _, f := range c.Fields {
		if f.Method != nil {
			returnType := "void"
			if f.Method.Type != nil {
				returnType = f.Method.Type.Type
			}
			classAst.Methods[f.Method.Name] = &ast.Method{
				Name:       f.Method.Name,
				Type:       returnType,
				Visibility: f.Method.Visibility,
				Parent:     classAst,
				Unsafe:     tokens.HasAttribute(f.Method.Attributes, "unsafe"),
			}
		}
	}

	// If this is a generic class, register it and skip C code generation
	if len(c.TypeParams) > 0 {
		originModule := scope.GetRoot().Scope
		Generics.RegisterGenericClass(c.Name, c, originModule)
		Generics.GenericClassScopes[c.Name] = scope
		return
	}

	// Get parent scope info to add the struct definition
	info := CGetScopeInformation(scope)

	// Generate C struct definition
	impls.GenerateClassDef(scope, c, c.Name, nil)

	// Add to types list (already added by GenerateClassDef)
	_ = info

	// Process methods (they go into the functions list)
	for _, f := range c.Fields {
		if f.Method != nil {
			impls.NewMethod(classAst, f.Method)
		}
	}
}

// GenerateClassDef generates a C struct definition, optionally with type substitution
func (impls *CBackendImplementation) GenerateClassDef(scope *ast.Ast, c *tokens.Class, name string, typeArgs []string) {
	info := CGetScopeInformation(scope)

	// Get or create the class AST
	classAst, ok := scope.Classes[name]
	if !ok {
		classAst = &ast.Ast{
			Scope:        name,
			Parent:       scope,
			OriginModule: scope.GetRoot().Scope,
			SourceFile:   scope.GetSourceFile(),
		}
		classAst.Init(scope.ErrorScope)
		scope.Classes[name] = classAst
	}

	var structDef string
	dependencies := []string{}
	valueDependencies := []string{}

	// Check for packed attribute
	isPacked := tokens.IsPacked(c.Attributes)
	if isPacked {
		structDef = "typedef struct __attribute__((packed)) {\n"
	} else {
		structDef = "typedef struct {\n"
	}

	// Add fields with type substitution
	for _, f := range c.Fields {
		if f.Field != nil {
			var fieldDecl string
			var cTypeForTracking string

			// Handle fixed-size arrays specially: [N]T -> T name[N]
			if f.Field.Type != nil && f.Field.Type.Size != nil {
				// Note: Don't validate field types here - circular dependency detection
				// needs forward references to work. Field types are validated at usage time.
				elemType := TypeRefToCType(f.Field.Type.Size.Type, scope)
				if len(typeArgs) > 0 && len(c.TypeParams) > 0 {
					elemType = SubstituteTypeParams(elemType, c.TypeParams, typeArgs)
				}
				fieldDecl = "    " + elemType + " " + f.Field.Name + "[" + f.Field.Type.Size.Size + "];\n"
				cTypeForTracking = elemType + "[" + f.Field.Type.Size.Size + "]"
				// Track dependency on array element type
				if depType := extractDependencyType(elemType); depType != "" {
					dependencies = append(dependencies, depType)
				}
				// Arrays of values create value dependencies
				if valDepType := extractValueDependencyType(elemType); valDepType != "" {
					valueDependencies = append(valueDependencies, valDepType)
				}
			} else {
				// Note: Don't validate field types here - circular dependency detection
				// needs forward references to work. Field types are validated at usage time.
				fieldType := TypeRefToCType(f.Field.Type, scope)
				// Substitute type parameters if this is a generic instantiation
				if len(typeArgs) > 0 && len(c.TypeParams) > 0 {
					fieldType = SubstituteTypeParams(fieldType, c.TypeParams, typeArgs)
				}
				fieldDecl = "    " + fieldType + " " + f.Field.Name + ";\n"
				cTypeForTracking = fieldType
				// Track dependency on field type (including pointer types, since we use typedefs)
				if depType := extractDependencyType(fieldType); depType != "" {
					dependencies = append(dependencies, depType)
				}
				// Track value dependencies (non-pointer) for cycle detection
				if valDepType := extractValueDependencyType(fieldType); valDepType != "" {
					valueDependencies = append(valueDependencies, valDepType)
				}
			}
			structDef += fieldDecl

			// Register field in the class AST for type tracking
			fieldVariable := ast.Variable{
				Name:      f.Field.Name,
				IsPointer: f.Field.Type != nil && f.Field.Type.Pointer,
				IsConst:   bindingIsConst(f.Field),
				Parent:    classAst,
			}
			classAst.Variables[f.Field.Name] = fieldVariable

			// Store full type info in CProgramValues
			(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
				CType:     cTypeForTracking,
				GeckoType: f.Field.Type,
			}
		}
	}

	structDef += "} " + name + ";\n"

	// Add to both Types (for backward compat) and StructDefs (for sorting)
	info.Types = append(info.Types, structDef)
	info.StructDefs = append(info.StructDefs, &StructDefinition{
		Name:              name,
		Code:              structDef,
		Dependencies:      dependencies,
		ValueDependencies: valueDependencies,
		Pos:               c.Pos,
	})
}

// stripTypeQualifiers removes leading const/volatile qualifiers from a C type string
// (e.g. "const volatile Rectangle*" -> "Rectangle*").
func stripTypeQualifiers(cType string) string {
	cleanType := strings.TrimSpace(cType)
	for {
		trimmed := strings.TrimPrefix(cleanType, "volatile ")
		trimmed = strings.TrimPrefix(trimmed, "const ")
		if trimmed == cleanType {
			break
		}
		cleanType = trimmed
	}
	return cleanType
}

// extractDependencyType extracts a type name from a C type string that we depend on.
// Returns empty string for primitives. Strips pointer suffixes since typedefs need to exist.
func extractDependencyType(cType string) string {
	// Skip primitive C types
	primitives := map[string]bool{
		"void": true, "int": true, "int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"float": true, "double": true, "const char*": true, "char": true,
	}

	// Remove const/volatile qualifiers (they appear before the base type, e.g. "const Rectangle*")
	cleanType := stripTypeQualifiers(cType)

	// For pointers, extract the base type - typedefs require the typedef to exist
	// even when used as a pointer (e.g., Rectangle* requires Rectangle typedef)
	for strings.HasSuffix(cleanType, "*") {
		cleanType = strings.TrimSuffix(cleanType, "*")
		cleanType = strings.TrimSpace(cleanType)
	}

	if primitives[cleanType] {
		return ""
	}

	return cleanType
}

// extractValueDependencyType extracts a type name only for VALUE (non-pointer) dependencies.
// Returns empty string for primitives and pointer types.
// Used for circular dependency detection - pointer cycles are allowed, value cycles are not.
func extractValueDependencyType(cType string) string {
	// Skip primitive C types
	primitives := map[string]bool{
		"void": true, "int": true, "int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"float": true, "double": true, "const char*": true, "char": true,
	}

	// Remove const/volatile qualifiers
	cleanType := stripTypeQualifiers(cType)

	// Pointers don't create value dependencies (they have fixed size)
	if strings.HasSuffix(cleanType, "*") {
		return ""
	}

	if primitives[cleanType] {
		return ""
	}

	return cleanType
}
