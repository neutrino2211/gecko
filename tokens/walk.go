package tokens

import "reflect"

func WalkSyntaxEntries(entries []*Entry, visit func(any)) {
	seen := make(map[any]bool)
	packagePath := reflect.TypeOf(Entry{}).PkgPath()
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != packagePath {
				return
			}
			node := value.Interface()
			if seen[node] {
				return
			}
			seen[node] = true
			visit(node)
			walk(value.Elem())
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					walk(value.Field(index))
				}
			}
		case reflect.Slice:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index))
			}
		}
	}
	walk(reflect.ValueOf(entries))
}
