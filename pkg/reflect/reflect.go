package reflect

import "reflect"

func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)

	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Slice, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
