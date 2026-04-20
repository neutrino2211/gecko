package tokens

import "strings"

// HasAttribute checks if the given list of attributes contains an attribute with the given name
func HasAttribute(attrs []*Attribute, name string) bool {
	for _, attr := range attrs {
		if attr.Name == name {
			return true
		}
	}
	return false
}

// GetAttribute returns the attribute with the given name, or nil if not found
func GetAttribute(attrs []*Attribute, name string) *Attribute {
	for _, attr := range attrs {
		if attr.Name == name {
			return attr
		}
	}
	return nil
}

// GetAttributeValue returns the value of an attribute, stripping quotes if present
func GetAttributeValue(attrs []*Attribute, name string) string {
	attr := GetAttribute(attrs, name)
	if attr == nil {
		return ""
	}
	return attr.GetStringValue()
}

// IsPacked checks if the attributes contain @packed
func IsPacked(attrs []*Attribute) bool {
	return HasAttribute(attrs, "packed")
}

// GetSection returns the section name from @section attribute, or empty string if not present
func GetSection(attrs []*Attribute) string {
	return GetAttributeValue(attrs, "section")
}

// GetAligned returns the alignment value from @aligned attribute, or empty string if not present
func GetAligned(attrs []*Attribute) string {
	return GetAttributeValue(attrs, "aligned")
}

// ToCAttributes converts gecko attributes to C __attribute__ syntax
func ToCAttributes(attrs []*Attribute) string {
	if len(attrs) == 0 {
		return ""
	}

	var cAttrs []string

	for _, attr := range attrs {
		switch attr.Name {
		case "packed":
			cAttrs = append(cAttrs, "__attribute__((packed))")
		case "section":
			value := attr.GetStringValue()
			cAttrs = append(cAttrs, "__attribute__((section(\""+value+"\")))")
		case "aligned":
			value := attr.GetStringValue()
			cAttrs = append(cAttrs, "__attribute__((aligned("+value+")))")
		case "used":
			cAttrs = append(cAttrs, "__attribute__((used))")
		case "noreturn":
			cAttrs = append(cAttrs, "__attribute__((noreturn))")
		case "naked":
			cAttrs = append(cAttrs, "__attribute__((naked))")
		}
	}

	return strings.Join(cAttrs, " ")
}
