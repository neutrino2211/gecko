// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tokens

func (i *ImplementationField) ToMethodToken() *Method {
	return &Method{
		baseToken:  i.baseToken, // Copy position info
		Name:       i.Name,
		Visibility: i.Visibility, // Preserve actual visibility (empty = private by default)
		Value:      i.Value,
		Type:       i.Type,
		Arguments:  i.Arguments,
	}
}
