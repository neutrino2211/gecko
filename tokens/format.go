package tokens

import (
	"fmt"
	"strings"
)

func FormatTypeRef(t *TypeRef) string {
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
		if t.FuncType.Throws != nil {
			sb.WriteString(" throws ")
			sb.WriteString(FormatTypeRef(t.FuncType.Throws))
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

	if t.Trait != "" {
		sb.WriteString(" is ")
		sb.WriteString(t.Trait)
	}
	if t.Const {
		sb.WriteString(" readonly")
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
