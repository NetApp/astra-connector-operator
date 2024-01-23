package util

import (
	"reflect"
	"regexp"
	"strings"
)

// dns1123LabelMaxLength is the maximum possible length of a Kubernetes resource label.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
const dns1123LabelMaxLength = 63

var dns1123Regex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// IsNil Return true if the interface value is actually nil, or the value is a nil pointer.
// In golang an interface value is equal to nil only if both it's value and type are nil.
// See:  https://stackoverflow.com/questions/13476349/check-for-nil-and-nil-interface-in-go
func IsNil(i interface{}) bool {
	return i == nil || (reflect.ValueOf(i).Kind() == reflect.Ptr && reflect.ValueOf(i).IsNil())
}

// GetJSONFieldName Utility function which returns the Struct Field JSON tag for the provided reference to the Struct field;
// If it is not a valid struct field, an empty string will be returned
func GetJSONFieldName(sourceStruct interface{}, fieldPointer interface{}) string {
	sourceValue := reflect.ValueOf(sourceStruct).Elem()
	requiredValue := reflect.ValueOf(fieldPointer).Elem()

	for i := 0; i < sourceValue.NumField(); i++ {
		valueField := sourceValue.Field(i)
		if valueField.Addr().Interface() == requiredValue.Addr().Interface() {
			jsonTag := sourceValue.Type().Field(i).Tag.Get("json")
			parts := strings.Split(jsonTag, ",")
			return parts[0]
		}
	}

	return ""
}

// IsValidDNS1123Label checks if the supplied string satisfies Kubernetes label requirements.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
func IsValidDNS1123Label(label string) bool {
	if len(label) > dns1123LabelMaxLength {
		return false
	}
	return dns1123Regex.MatchString(label)
}
