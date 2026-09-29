package tokens

import "reflect"

type SyntaxCloner struct{ nodes map[any]any }

func NewSyntaxCloner() *SyntaxCloner { return &SyntaxCloner{nodes: make(map[any]any)} }

func (c *SyntaxCloner) Nodes() map[any]any { return c.nodes }

func (c *SyntaxCloner) File(file *File) *File {
	if file == nil {
		return nil
	}
	return c.clone(reflect.ValueOf(file)).Interface().(*File)
}

func (c *SyntaxCloner) Expression(expression *Expression) *Expression {
	if expression == nil {
		return nil
	}
	return c.clone(reflect.ValueOf(expression)).Interface().(*Expression)
}

func (c *SyntaxCloner) clone(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() || value.Type().Elem().PkgPath() != reflect.TypeOf(File{}).PkgPath() {
			return value
		}
		if found, ok := c.nodes[value.Interface()]; ok {
			return reflect.ValueOf(found)
		}
		copy := reflect.New(value.Type().Elem())
		c.nodes[value.Interface()] = copy.Interface()
		copy.Elem().Set(c.clone(value.Elem()))
		return copy
	case reflect.Struct:
		copy := reflect.New(value.Type()).Elem()
		copy.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				copy.Field(i).Set(c.clone(value.Field(i)))
			}
		}
		return copy
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		copy := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copy.Index(i).Set(c.clone(value.Index(i)))
		}
		return copy
	default:
		return value
	}
}
