package runtime

import "reflect"

// IsZeroValue returns whether v should be treated as absent by generated CRUD
// validation. Nullable structs are checked by a Valid boolean field regardless
// of presence of other fields.
func IsZeroValue(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct {
		if valid := rv.FieldByName("Valid"); valid.IsValid() && valid.Kind() == reflect.Bool {
			return !valid.Bool()
		}
	}

	return rv.IsZero()
}
