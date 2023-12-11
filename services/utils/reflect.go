package utils

import "reflect"

// REturn list of all fields in a struct, recursively by embedded structs
func StructFields(s interface{}) []reflect.StructField {
	var fields []reflect.StructField
	structFieldsRec(s, &fields)
	return fields
}

func structFieldsRec(s interface{}, fields *[]reflect.StructField) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			structFieldsRec(v.Field(i).Interface(), fields)
		} else {
			*fields = append(*fields, field)
		}
	}
}

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
