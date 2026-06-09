// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/modules.md, spec/scoping.md

package ast

import (
	"strings"
)

type Method struct {
	Name           string
	Arguments      []Variable
	Scope          *Ast
	Visibility     string
	Parent         *Ast
	Type           string
	ExternalSymbol string
	Throws         string // Error type this method can throw (empty if none)
}

func (m *Method) GetFullName() string {
	cString := ""

	if m.Visibility == "external" {
		cString = m.Name
	} else {
		cString = strings.ReplaceAll(m.Parent.FullScopeName()+"."+m.Name, ".", "__")
	}

	return cString
}

func (m *Method) CIdentifier() string {
	if m == nil {
		return ""
	}
	if m.ExternalSymbol != "" {
		return m.ExternalSymbol
	}
	return m.GetFullName()
}

func (m *Method) ToCString() string {
	content := m.Scope.ToCString()

	return m.Type + " " + m.Name + "() {\n" + content + "}"
}

// IsPublic returns true if this method is accessible from anywhere
func (m *Method) IsPublic() bool {
	return m.Visibility == "public" || m.Visibility == "external"
}

// IsProtected returns true if this method is accessible within the same package
func (m *Method) IsProtected() bool {
	return m.Visibility == "protected"
}

// IsPrivate returns true if this method is only accessible within the same file
func (m *Method) IsPrivate() bool {
	return m.Visibility == "private" || m.Visibility == ""
}

// GetOriginModule returns the package where this method was defined
func (m *Method) GetOriginModule() string {
	if m.Parent != nil {
		return m.Parent.GetOriginModule()
	}
	return ""
}

// GetSourceFile returns the source file where this method was defined
func (m *Method) GetSourceFile() string {
	if m.Parent != nil {
		return m.Parent.GetSourceFile()
	}
	return ""
}

// IsSameFile checks if this method is from the same file as the given scope
func (m *Method) IsSameFile(fromScope *Ast) bool {
	if m.Parent == nil || fromScope == nil {
		return false
	}
	return m.GetSourceFile() == fromScope.GetSourceFile()
}

// IsSamePackage checks if this method is from the same package as the given scope
func (m *Method) IsSamePackage(fromScope *Ast) bool {
	if fromScope == nil {
		return false
	}
	return m.GetOriginModule() == fromScope.GetOriginModule()
}

// CheckVisibility validates that a method can be accessed from the given scope.
// Returns an error message if access is denied, empty string if allowed.
// Visibility levels:
//   - private (default): same file only
//   - protected: same package (any file in package)
//   - public/external: accessible from anywhere
func (m *Method) CheckVisibility(fromScope *Ast) string {
	// Public/external methods are always accessible
	if m.IsPublic() {
		return ""
	}

	// Protected methods are accessible within the same package
	if m.IsProtected() {
		if m.IsSamePackage(fromScope) {
			return ""
		}
		return "method '" + m.Name + "' is protected and can only be accessed within package '" + m.GetOriginModule() + "'"
	}

	// Private methods (default) are only accessible within the same file
	if m.IsSameFile(fromScope) {
		return ""
	}

	visibility := m.Visibility
	if visibility == "" {
		visibility = "private (default)"
	}
	return "method '" + m.Name + "' is " + visibility + " and can only be accessed within the same file"
}
