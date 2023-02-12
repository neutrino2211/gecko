package tokens

func (i *ImplementationField) ToMethodToken() *Method {
	return &Method{
		Name:       i.Name,
		Visibility: "public",
		Value:      i.Value,
		Type:       i.Type,
		Arguments:  i.Arguments,
	}
}
