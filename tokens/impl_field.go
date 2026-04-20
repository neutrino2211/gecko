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
