package types

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/stretchr/testify/assert"
)

// dateTimestampPartFullName is the proto FullName for ZetaSQL's
// DateTimestampPart enum, registered in protoregistry.GlobalTypes by
// wasm/generated/zetasql_public_functions_datetime.pb.go's init().
// It is the canonical witness used throughout these tests because it
// is a builtin enum the downstream emulator hits via INTERVAL N PART.
const dateTimestampPartFullName = "zetasql.functions.DateTimestampPart"

func TestNewEnumType(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    *EnumType
		wantErr bool
	}{
		{
			name: "fully-qualified name yields *EnumType with that name",
			in:   dateTimestampPartFullName,
			want: &EnumType{Name: dateTimestampPartFullName},
		},
		{
			name:    "empty name is rejected (no way to dispatch NameOf)",
			in:      "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.in

			// Act
			got, err := NewEnumType(sut)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEnumType_TypePredicates pins the *EnumType implementation of the
// Type interface: a single fixture, all six predicate/cast methods
// observed in one go so a regression in any single method shows up in
// the diff.
func TestEnumType_TypePredicates(t *testing.T) {
	// Arrange
	sut := &EnumType{Name: dateTimestampPartFullName}

	// Act / Assert
	assert.Equal(t, Enum, sut.Kind())
	assert.False(t, sut.IsArray())
	assert.False(t, sut.IsStruct())
	assert.True(t, sut.IsEnum())
	assert.Nil(t, sut.AsArray())
	assert.Nil(t, sut.AsStruct())
	assert.Equal(t, sut, sut.AsEnum())
}

// TestEnumType_ToProto_RoundTrip pairs ToProto with TypeFromProto so
// the proto round-trip lands back on a value that compares equal to
// the original. Going through the round-trip (instead of asserting on
// the proto wire shape directly) keeps the test stable under proto
// generation churn while still observing both directions.
func TestEnumType_ToProto_RoundTrip(t *testing.T) {
	// Arrange
	sut := &EnumType{Name: dateTimestampPartFullName}

	// Act
	got, err := TypeFromProto(sut.ToProto())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, sut, got)
}

// TestEnumType_NameOf documents the four observable outcomes of
// NameOf in one table:
//
//   - registered enum, defined number → name returned with ok=true;
//   - registered enum, undefined number → ("", false);
//   - unregistered enum (descriptor not linked into the binary) →
//     ("", false);
//   - nil receiver → ("", false), the same shape as the typed
//     accessors so callers can chain through *EnumType from a
//     possibly-nil Type without a guard.
//
// DateTimestampPart's value table (DAY=3, HOUR=7, etc.) is the source
// of truth for the names; we pick DAY because it is the value the
// downstream INTERVAL N PART path actually exercises, and HOUR as a
// triangulation point so a regression that returned the same constant
// for every input would still fail.
func TestEnumType_NameOf(t *testing.T) {
	tests := []struct {
		name     string
		typ      *EnumType
		number   int32
		wantName string
		wantOK   bool
	}{
		{
			name:     "DateTimestampPart=3 resolves to DAY",
			typ:      &EnumType{Name: dateTimestampPartFullName},
			number:   int32(generated.DateTimestampPart_DAY),
			wantName: "DAY",
			wantOK:   true,
		},
		{
			name:     "DateTimestampPart=7 resolves to HOUR (triangulation)",
			typ:      &EnumType{Name: dateTimestampPartFullName},
			number:   int32(generated.DateTimestampPart_HOUR),
			wantName: "HOUR",
			wantOK:   true,
		},
		{
			name:     "registered enum but undefined number yields (\"\", false)",
			typ:      &EnumType{Name: dateTimestampPartFullName},
			number:   9999,
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "unregistered enum yields (\"\", false)",
			typ:      &EnumType{Name: "no.such.Enum"},
			number:   1,
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "nil receiver yields (\"\", false)",
			typ:      nil,
			number:   1,
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "empty Name yields (\"\", false)",
			typ:      &EnumType{Name: ""},
			number:   1,
			wantName: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.typ

			// Act
			gotName, gotOK := sut.NameOf(tt.number)

			// Assert
			assert.Equal(t, tt.wantName, gotName)
			assert.Equal(t, tt.wantOK, gotOK)
		})
	}
}

