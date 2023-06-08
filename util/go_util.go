package util

import "reflect"

// IsNil Return true if the interface value is actually nil, or the value is a nil pointer.
// In golang an interface value is equal to nil only if both it's value and type are nil.
// See:  https://stackoverflow.com/questions/13476349/check-for-nil-and-nil-interface-in-go
func IsNil(i interface{}) bool {
	return i == nil || (reflect.ValueOf(i).Kind() == reflect.Ptr && reflect.ValueOf(i).IsNil())
}
