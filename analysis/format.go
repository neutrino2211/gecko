// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package analysis

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

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
