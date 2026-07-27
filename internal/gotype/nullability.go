package gotype

import "strings"

// ApplyNullability adds or removes a pointer from the string type.
func ApplyNullability(typeName string, nullable bool) string {
	if typeName == "" {
		typeName = "any"
	}
	if nullable {
		if strings.HasPrefix(typeName, "*") {
			return typeName
		}
		return "*" + typeName
	}
	return strings.TrimPrefix(typeName, "*")
}
