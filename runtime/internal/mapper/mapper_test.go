package mapper

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestDecodeJSONAcceptsNaiveTimestampStrings(t *testing.T) {
	type artist struct {
		Name      string     `json:"name"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at"`
	}

	raw := []byte(`[
		{
			"name": "Example Artist",
			"created_at": "2026-01-27T23:33:42.015",
			"updated_at": "2026-01-28T01:02:03.004"
		}
	]`)

	var artists []artist
	fieldValue := reflect.ValueOf(&artists).Elem()
	if err := decodeJSON(fieldValue, fieldValue.Type(), "artists", &raw); err != nil {
		t.Fatalf("decodeJSON() error = %v", err)
	}

	if len(artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(artists))
	}
	if artists[0].Name != "Example Artist" {
		t.Fatalf("unexpected artist name %q", artists[0].Name)
	}
	wantCreated := time.Date(2026, 1, 27, 23, 33, 42, 15_000_000, time.UTC)
	if !artists[0].CreatedAt.Equal(wantCreated) {
		t.Fatalf("created_at = %v want %v", artists[0].CreatedAt, wantCreated)
	}
	if artists[0].UpdatedAt == nil {
		t.Fatalf("expected updated_at to be set")
	}
	wantUpdated := time.Date(2026, 1, 28, 1, 2, 3, 4_000_000, time.UTC)
	if !artists[0].UpdatedAt.Equal(wantUpdated) {
		t.Fatalf("updated_at = %v want %v", artists[0].UpdatedAt, wantUpdated)
	}
}

func TestBuildMetadataTreatsQuerycJSONStructAsLeafField(t *testing.T) {
	type stats struct {
		Count    *uint64 `json:"count"`
		Duration *uint64 `json:"duration"`
	}
	type row struct {
		Current stats `db:"current" queryc:"json"`
	}

	meta := buildMetadata(reflect.TypeFor[row](), StandardHooks{})
	mappings := meta.fieldsByColumn["current"]
	if len(mappings) != 1 {
		t.Fatalf("expected one mapping for current, got %d", len(mappings))
	}
	if mappings[0].querycCodec != "json" {
		t.Fatalf("expected current mapping to use json codec, got %q", mappings[0].querycCodec)
	}
	if _, ok := meta.fieldsByColumn["count"]; ok {
		t.Fatalf("did not expect nested json field count to be mapped as top-level column")
	}
}

func TestGetTableNameFromFieldUsesSnakePluralAndTag(t *testing.T) {
	type UserConfiguration struct{}
	tagged := reflect.StructField{
		Name: "UserConfiguration",
		Type: reflect.TypeFor[UserConfiguration](),
	}
	if got := getTableNameFromField(tagged); got != "user_configurations" {
		t.Fatalf("fallback table name = %q, want user_configurations", got)
	}
	tagged.Tag = `table:"user_configurations"`
	if got := getTableNameFromField(tagged); got != "user_configurations" {
		t.Fatalf("tagged table name = %q", got)
	}
}

func TestUnwrapAndAssignLeavesAllNullEmbeddedPointerNil(t *testing.T) {
	type profile struct {
		Name string `db:"name"`
	}
	type row struct {
		Profile *profile `table:"profiles"`
	}

	meta := buildMetadata(reflect.TypeFor[row](), StandardHooks{})
	var result row
	targets, mappings := buildScanTargets(&result, []string{"name"}, meta, StandardHooks{})
	target := targets[0].(*sql.NullString)

	if err := unwrapAndAssign(&result, targets, mappings, StandardHooks{}); err != nil {
		t.Fatalf("unwrapAndAssign() error = %v", err)
	}
	if result.Profile != nil {
		t.Fatal("all-NULL embedded pointer must remain nil")
	}

	target.String = "Alex"
	target.Valid = true
	if err := unwrapAndAssign(&result, targets, mappings, StandardHooks{}); err != nil {
		t.Fatalf("unwrapAndAssign() error = %v", err)
	}
	if result.Profile == nil || result.Profile.Name != "Alex" {
		t.Fatalf("non-NULL embedded value was not assigned: %#v", result.Profile)
	}
}

func TestUnwrapAndAssignRejectsNullForNonNullableField(t *testing.T) {
	type row struct {
		Name string `db:"name"`
	}

	meta := buildMetadata(reflect.TypeFor[row](), StandardHooks{})
	var result row
	targets, mappings := buildScanTargets(&result, []string{"name"}, meta, StandardHooks{})
	if err := unwrapAndAssign(&result, targets, mappings, StandardHooks{}); err == nil {
		t.Fatal("expected NULL assigned to string field to fail")
	}
}

func TestMetadataCacheIncludesHooksType(t *testing.T) {
	type nested struct {
		Value string
	}
	type row struct {
		Nested nested
	}

	special, err := getOrBuildMetadata(reflect.TypeFor[row](), specialStructHooks{})
	if err != nil {
		t.Fatalf("getOrBuildMetadata() error = %v", err)
	}
	ordinary, err := getOrBuildMetadata(reflect.TypeFor[row](), StandardHooks{})
	if err != nil {
		t.Fatalf("getOrBuildMetadata() error = %v", err)
	}

	if _, ok := special.fieldsByColumn["nested"]; !ok {
		t.Fatal("special hooks should map the nested struct as a leaf")
	}
	if _, ok := ordinary.fieldsByColumn["value"]; !ok {
		t.Fatal("ordinary hooks should recursively map the nested struct")
	}
}

func TestGetOrBuildMetadataRejectsNonStructResults(t *testing.T) {
	if _, err := getOrBuildMetadata(reflect.TypeFor[int](), StandardHooks{}); err == nil {
		t.Fatal("expected non-struct result type to be rejected")
	}
}

type scanRows struct {
	values [][]any
	index  int
}

func (r *scanRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *scanRows) Scan(dest ...any) error {
	for i, value := range r.values[r.index-1] {
		if scanner, ok := dest[i].(sql.Scanner); ok {
			if err := scanner.Scan(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (*scanRows) Err() error { return nil }

func TestScanAll(t *testing.T) {
	type row struct {
		Name string `db:"name"`
	}
	rows := &scanRows{values: [][]any{{"first"}, {"second"}}}
	got, err := ScanAll[row](rows, []string{"name"}, StandardHooks{})
	if err != nil {
		t.Fatalf("ScanAll() error = %v", err)
	}
	if len(got) != 2 || got[0].Name != "first" || got[1].Name != "second" {
		t.Fatalf("ScanAll() = %#v", got)
	}
}

type specialStructHooks struct{ StandardHooks }

func (specialStructHooks) IsSpecialStruct(reflect.Type) bool {
	return true
}
