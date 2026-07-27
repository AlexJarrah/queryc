package mapper

import "reflect"

// StandardHooks implements Hooks for dialects that only need the standard
// nullable and special value tables.
type StandardHooks struct{}

// IsNullableType retursn whether t can represent SQL NULL.
func (StandardHooks) IsNullableType(t reflect.Type) bool {
	return IsStandardNullableType(t)
}

// IsSpecialStruct returns whether t requires direct struct scanning.
func (StandardHooks) IsSpecialStruct(t reflect.Type) bool {
	return IsStandardSpecialType(t)
}

// CreateSpecialScanTarget returns that no dialect-specific scan target exists.
func (StandardHooks) CreateSpecialScanTarget(reflect.Value, reflect.Type) (any, bool) {
	return nil, false
}

// AssignSpecialValue returns that no dialect-specific assignment exists.
func (StandardHooks) AssignSpecialValue(reflect.Value, any) (bool, error) {
	return false, nil
}
