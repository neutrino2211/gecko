package ast

import (
	"strings"
)

type Method struct {
	Name       string
	Arguments  []Variable
	Scope      *Ast
	Visibility string
	Parent     *Ast
	Type       string
	Throws     string // Error type this method can throw (empty if none)
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

func (m *Method) ToCString() string {
	content := m.Scope.ToCString()

	return m.Type + " " + m.Name + "() {\n" + content + "}"
}

// IsPublic returns true if this method is accessible from other modules
func (m *Method) IsPublic() bool {
	return m.Visibility == "public" || m.Visibility == "external"
}

// GetOriginModule returns the module where this method was defined
func (m *Method) GetOriginModule() string {
	if m.Parent != nil {
		return m.Parent.GetRoot().Scope
	}
	return ""
}

// CheckVisibility validates that a method can be accessed from the given scope.
// Returns an error message if access is denied, empty string if allowed.
func (m *Method) CheckVisibility(fromScope *Ast) string {
	// External methods are always accessible
	if m.Visibility == "external" {
		return ""
	}

	// Same module access is always allowed
	fromModule := fromScope.GetRoot().Scope
	if m.GetOriginModule() == fromModule {
		return ""
	}

	// Cross-module access requires public visibility
	if !m.IsPublic() {
		visibility := m.Visibility
		if visibility == "" {
			visibility = "private (default)"
		}
		return "method '" + m.Name + "' is " + visibility + " and cannot be accessed from module '" + fromModule + "'"
	}

	return ""
}
