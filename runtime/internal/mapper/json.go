package mapper

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func decodeJSON(fieldValue reflect.Value, fieldType reflect.Type, columnName string, target any) error {
	var raw []byte
	switch v := target.(type) {
	case *[]byte:
		raw = *v
	case *string:
		raw = []byte(*v)
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("failed to decode queryc json column %s: unsupported target %T", columnName, target)
	}
	if len(raw) == 0 {
		return nil
	}

	decodeTarget := fieldValue
	if fieldType.Kind() == reflect.Pointer {
		if fieldValue.IsNil() {
			fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
		}
	} else {
		decodeTarget = fieldValue.Addr()
	}

	if err := json.Unmarshal(raw, decodeTarget.Interface()); err != nil {
		var generic any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decodeErr := decoder.Decode(&generic); decodeErr != nil {
			return fmt.Errorf("failed to decode queryc json column %s: %w", columnName, decodeErr)
		}
		if assignErr := assignJSONValue(reflect.Indirect(decodeTarget), generic); assignErr != nil {
			return fmt.Errorf("failed to decode queryc json column %s: %w", columnName, assignErr)
		}
	}
	return nil
}

func assignJSONValue(target reflect.Value, data any) error {
	if !target.IsValid() {
		return nil
	}

	if target.Kind() == reflect.Pointer {
		if data == nil {
			return nil
		}
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		return assignJSONValue(target.Elem(), data)
	}

	if target.Type() == reflect.TypeFor[time.Time]() {
		text, ok := data.(string)
		if !ok {
			return fmt.Errorf("expected string timestamp for %s, got %T", target.Type(), data)
		}
		parsed, err := parseFlexibleTime(text)
		if err != nil {
			return err
		}
		target.Set(reflect.ValueOf(parsed))
		return nil
	}

	if target.CanAddr() {
		if unmarshaler, ok := target.Addr().Interface().(encoding.TextUnmarshaler); ok {
			text, ok := data.(string)
			if !ok {
				return fmt.Errorf("expected string for %s, got %T", target.Type(), data)
			}
			return unmarshaler.UnmarshalText([]byte(text))
		}
	}

	switch target.Kind() {
	case reflect.Struct:
		obj, ok := data.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object for %s, got %T", target.Type(), data)
		}
		fieldMap := jsonFieldMap(target.Type())
		for key, value := range obj {
			index, ok := fieldMap[key]
			if !ok {
				continue
			}
			if err := assignJSONValue(target.Field(index), value); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if data == nil {
			target.Set(reflect.MakeSlice(target.Type(), 0, 0))
			return nil
		}
		items, ok := data.([]any)
		if !ok {
			return fmt.Errorf("expected array for %s, got %T", target.Type(), data)
		}
		result := reflect.MakeSlice(target.Type(), len(items), len(items))
		for i, item := range items {
			if err := assignJSONValue(result.Index(i), item); err != nil {
				return err
			}
		}
		target.Set(result)
		return nil
	case reflect.Array:
		items, ok := data.([]any)
		if !ok {
			return fmt.Errorf("expected array for %s, got %T", target.Type(), data)
		}
		for i := 0; i < target.Len() && i < len(items); i++ {
			if err := assignJSONValue(target.Index(i), items[i]); err != nil {
				return err
			}
		}
		return nil
	case reflect.String:
		text, ok := data.(string)
		if !ok {
			return fmt.Errorf("expected string for %s, got %T", target.Type(), data)
		}
		target.SetString(text)
		return nil
	case reflect.Bool:
		boolean, ok := data.(bool)
		if !ok {
			return fmt.Errorf("expected bool for %s, got %T", target.Type(), data)
		}
		target.SetBool(boolean)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := jsonInt64(data)
		if err != nil {
			return err
		}
		if target.OverflowInt(value) {
			return fmt.Errorf("integer %d overflows %s", value, target.Type())
		}
		target.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := jsonUint64(data)
		if err != nil {
			return err
		}
		if target.OverflowUint(value) {
			return fmt.Errorf("unsigned integer %d overflows %s", value, target.Type())
		}
		target.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		value, err := jsonFloat64(data)
		if err != nil {
			return err
		}
		if target.OverflowFloat(value) {
			return fmt.Errorf("float %v overflows %s", value, target.Type())
		}
		target.SetFloat(value)
		return nil
	case reflect.Interface:
		target.Set(reflect.ValueOf(data))
		return nil
	}

	return fmt.Errorf("unsupported json assignment for %s", target.Type())
}

func jsonFieldMap(t reflect.Type) map[string]int {
	fields := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			name = strings.Split(tag, ",")[0]
		}
		if name == "" || name == "-" {
			continue
		}
		fields[name] = i
	}
	return fields
}

func parseFlexibleTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			if strings.Contains(layout, "Z07:00") {
				return parsed, nil
			}
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func jsonInt64(data any) (int64, error) {
	switch value := data.(type) {
	case json.Number:
		return value.Int64()
	case float64:
		return int64(value), nil
	case int64:
		return value, nil
	case int32:
		return int64(value), nil
	case int:
		return int64(value), nil
	case string:
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, fmt.Errorf("expected integer, got %T", data)
	}
}

func jsonUint64(data any) (uint64, error) {
	switch value := data.(type) {
	case json.Number:
		i, err := value.Int64()
		if err != nil {
			return 0, err
		}
		if i < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", i)
		}
		return uint64(i), nil
	case float64:
		if value < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %f", value)
		}
		return uint64(value), nil
	case uint64:
		return value, nil
	case uint32:
		return uint64(value), nil
	case uint:
		return uint64(value), nil
	case int64:
		if value < 0 {
			return 0, fmt.Errorf("expected unsigned integer, got %d", value)
		}
		return uint64(value), nil
	case string:
		return strconv.ParseUint(value, 10, 64)
	default:
		return 0, fmt.Errorf("expected unsigned integer, got %T", data)
	}
}

func jsonFloat64(data any) (float64, error) {
	switch value := data.(type) {
	case json.Number:
		return value.Float64()
	case float64:
		return value, nil
	case float32:
		return float64(value), nil
	case int64:
		return float64(value), nil
	case int:
		return float64(value), nil
	case string:
		return strconv.ParseFloat(value, 64)
	default:
		return 0, fmt.Errorf("expected float, got %T", data)
	}
}
