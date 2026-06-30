package core

import "reflect"

// TypeOf returns the reflect type of T.
func TypeOf[T any]() reflect.Type {
	var zero *T
	return reflect.TypeOf(zero).Elem()
}

// TypeName returns a stable type name string.
func TypeName(value reflect.Type) string {
	if value == nil {
		return ""
	}
	return value.String()
}
