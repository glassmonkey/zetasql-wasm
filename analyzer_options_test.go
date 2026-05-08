package zetasql

import (
	"sort"
	"testing"

	"github.com/glassmonkey/zetasql-wasm/resolved_ast"
	"github.com/glassmonkey/zetasql-wasm/types"
	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestAnalyzerOptions_toProto(t *testing.T) {
	fullScope := ParseLocationRecordFullNodeScope
	codeSearch := ParseLocationRecordCodeSearch
	allowUndecl := true
	fullScopeProto := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE
	codeSearchProto := generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_CODE_SEARCH
	positionalProto := generated.ParameterMode_PARAMETER_POSITIONAL
	noneProto := generated.ParameterMode_PARAMETER_NONE

	tests := []struct {
		name string
		opts *AnalyzerOptions
		want *generated.AnalyzerOptionsProto
	}{
		{
			name: "default options yield empty proto",
			opts: NewAnalyzerOptions(),
			want: &generated.AnalyzerOptionsProto{},
		},
		{
			name: "ParseLocationRecordType FULL_NODE_SCOPE is propagated",
			opts: &AnalyzerOptions{ParseLocationRecordType: &fullScope},
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &fullScopeProto},
		},
		{
			name: "ParseLocationRecordType CODE_SEARCH is propagated",
			opts: &AnalyzerOptions{ParseLocationRecordType: &codeSearch},
			want: &generated.AnalyzerOptionsProto{ParseLocationRecordType: &codeSearchProto},
		},
		{
			name: "AllowUndeclaredParameters true is propagated",
			opts: &AnalyzerOptions{AllowUndeclaredParameters: true},
			want: &generated.AnalyzerOptionsProto{AllowUndeclaredParameters: &allowUndecl},
		},
		{
			name: "ParameterMode POSITIONAL is propagated",
			opts: &AnalyzerOptions{ParameterMode: ParameterPositional},
			want: &generated.AnalyzerOptionsProto{ParameterMode: &positionalProto},
		},
		{
			name: "ParameterMode NONE is propagated",
			opts: &AnalyzerOptions{ParameterMode: ParameterNone},
			want: &generated.AnalyzerOptionsProto{ParameterMode: &noneProto},
		},
		{
			name: "ParameterMode NAMED (zero) is omitted from proto",
			opts: &AnalyzerOptions{ParameterMode: ParameterNamed},
			want: &generated.AnalyzerOptionsProto{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.opts

			// Act
			got := sut.toProto()

			// Assert
			assert.Empty(t, cmp.Diff(tt.want, got, protocmp.Transform()))
		})
	}
}

// TestAnalyzerOptions_QueryParameters_toProto verifies that named
// parameter types reach the wire form. Go map iteration is randomised,
// so the produced QueryParameters slice is sorted by Name before the
// proto diff to avoid order-induced flakes. Triangulated across nil /
// single / two-named / nil-Type-skipped so a regression in any single
// part of the wiring shows up in the diff.
func TestAnalyzerOptions_QueryParameters_toProto(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]types.Type
		want []*generated.AnalyzerOptionsProto_QueryParameterProto
	}{
		{
			name: "nil map yields no QueryParameters on the wire",
			in:   nil,
			want: nil,
		},
		{
			name: "single named INT64 parameter is propagated",
			in:   map[string]types.Type{"id": types.Int64Type()},
			want: []*generated.AnalyzerOptionsProto_QueryParameterProto{
				{Name: proto.String("id"), Type: types.Int64Type().ToProto()},
			},
		},
		{
			name: "two named parameters round-trip with type preserved",
			in: map[string]types.Type{
				"name": types.StringType(),
				"id":   types.Int64Type(),
			},
			want: []*generated.AnalyzerOptionsProto_QueryParameterProto{
				{Name: proto.String("id"), Type: types.Int64Type().ToProto()},
				{Name: proto.String("name"), Type: types.StringType().ToProto()},
			},
		},
		{
			name: "entry with nil Type is skipped",
			in: map[string]types.Type{
				"id":      types.Int64Type(),
				"skipped": nil,
			},
			want: []*generated.AnalyzerOptionsProto_QueryParameterProto{
				{Name: proto.String("id"), Type: types.Int64Type().ToProto()},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &AnalyzerOptions{QueryParameters: tt.in}

			// Act
			got := sut.toProto().GetQueryParameters()
			sort.Slice(got, func(i, j int) bool { return got[i].GetName() < got[j].GetName() })

			// Assert
			assert.Empty(t, cmp.Diff(tt.want, got, protocmp.Transform()))
		})
	}
}

// TestAnalyzerOptions_PositionalQueryParameters_toProto verifies that
// positional parameter types reach the wire form, in the same order
// the caller supplied them, with nil entries skipped per
// AnalyzerOptions's documented contract.
func TestAnalyzerOptions_PositionalQueryParameters_toProto(t *testing.T) {
	tests := []struct {
		name string
		in   []types.Type
		want []*generated.TypeProto
	}{
		{
			name: "nil slice yields no PositionalQueryParameters on the wire",
			in:   nil,
			want: nil,
		},
		{
			name: "single positional INT64 parameter is propagated",
			in:   []types.Type{types.Int64Type()},
			want: []*generated.TypeProto{types.Int64Type().ToProto()},
		},
		{
			name: "two positional parameters preserve order",
			in:   []types.Type{types.Int64Type(), types.StringType()},
			want: []*generated.TypeProto{types.Int64Type().ToProto(), types.StringType().ToProto()},
		},
		{
			name: "nil entry mid-slice is skipped (successors shift down)",
			in:   []types.Type{types.Int64Type(), nil, types.StringType()},
			want: []*generated.TypeProto{types.Int64Type().ToProto(), types.StringType().ToProto()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := &AnalyzerOptions{PositionalQueryParameters: tt.in}

			// Act
			got := sut.toProto().GetPositionalQueryParameters()

			// Assert
			assert.Empty(t, cmp.Diff(tt.want, got, protocmp.Transform()))
		})
	}
}

// TestAnalyzerOptions_Clone verifies that Clone returns a deep copy whose
// fields equal the original by value. Pointer-independence is checked in a
// separate test so each behaviour has a single observable assertion.
func TestAnalyzerOptions_Clone(t *testing.T) {
	fullScope := ParseLocationRecordFullNodeScope

	tests := []struct {
		name string
		opts *AnalyzerOptions
		want *AnalyzerOptions
	}{
		{
			name: "empty options clone equals empty options",
			opts: &AnalyzerOptions{},
			want: &AnalyzerOptions{},
		},
		{
			name: "populated options clone equals original by value",
			opts: &AnalyzerOptions{
				Language: &LanguageOptions{
					Features: map[LanguageFeature]bool{
						FeatureAnalyticFunctions: true,
					},
					StatementKinds: []StatementKind{StatementKindQuery},
					AllStatements:  false,
					Keywords:       map[string]bool{"QUALIFY": true},
				},
				ParseLocationRecordType: &fullScope,
			},
			want: &AnalyzerOptions{
				Language: &LanguageOptions{
					Features: map[LanguageFeature]bool{
						FeatureAnalyticFunctions: true,
					},
					StatementKinds: []StatementKind{StatementKindQuery},
					AllStatements:  false,
					Keywords:       map[string]bool{"QUALIFY": true},
				},
				ParseLocationRecordType: &fullScope,
			},
		},
		{
			name: "QueryParameters and PositionalQueryParameters round-trip",
			opts: &AnalyzerOptions{
				QueryParameters: map[string]types.Type{
					"id": types.Int64Type(),
				},
				PositionalQueryParameters: []types.Type{types.StringType()},
			},
			want: &AnalyzerOptions{
				QueryParameters: map[string]types.Type{
					"id": types.Int64Type(),
				},
				PositionalQueryParameters: []types.Type{types.StringType()},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.opts

			// Act
			got := sut.Clone()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestAnalyzerOptions_Clone_doesNotShareLanguagePointer verifies that the
// Clone method produces an AnalyzerOptions whose Language field is a distinct
// pointer from the original — mutating the clone's Language must not leak
// into the original.
func TestAnalyzerOptions_Clone_doesNotShareLanguagePointer(t *testing.T) {
	// Arrange
	sut := &AnalyzerOptions{
		Language: &LanguageOptions{
			Features: map[LanguageFeature]bool{
				FeatureAnalyticFunctions: true,
			},
			Keywords: map[string]bool{"QUALIFY": true},
		},
	}

	// Act
	clone := sut.Clone()
	clone.Language.Features[FeatureTablesample] = true
	clone.Language.Keywords["OFFSET"] = true
	got := sut.Language

	// Assert
	want := &LanguageOptions{
		Features: map[LanguageFeature]bool{
			FeatureAnalyticFunctions: true,
		},
		Keywords: map[string]bool{"QUALIFY": true},
	}
	assert.Equal(t, want, got)
}

// TestAnalyzerOptions_NamedQueryParameters_AnalyzerIntegration covers
// the named-parameter analyzer path end to end: the wire fields
// QueryParameters / ParameterMode / AllowUndeclaredParameters all
// round-trip through the WASM bridge into the C++ AnalyzerOptions and
// shape the analyzer's strict-vs-permissive decision.
//
// Triangulated across:
//   - declared @id (INT64) and @label (STRING) resolve cleanly,
//   - undeclared @nope rejects under the strict default,
//   - the same @nope is accepted when AllowUndeclaredParameters=true.
//
// The (got, err) tuple is checked in both halves: an error case
// carries a nil output, a happy case carries a non-nil output.
//
// Regression for the v0.5.0 emulator boot bug — `SELECT @id` failed
// analysis with "Query parameter 'X' not found" because the Go wrap
// had no QueryParameters field and the WASM bridge dropped the proto
// fields even when populated.
func TestAnalyzerOptions_NamedQueryParameters_AnalyzerIntegration(t *testing.T) {
	tests := []struct {
		name                      string
		params                    map[string]types.Type
		allowUndeclaredParameters bool
		sql                       string
		// wantParam is the observable shape of the resolved tree's
		// first ParameterNode. Ignored when wantErr is non-nil.
		wantParam observedParam
		wantErr   error
	}{
		{
			name:      "INT64 parameter @id resolves to declared INT64",
			params:    map[string]types.Type{"id": types.Int64Type()},
			sql:       "SELECT @id",
			wantParam: observedParam{TypeKind: generated.TypeKind_TYPE_INT64},
		},
		{
			name:      "STRING parameter @label resolves to declared STRING",
			params:    map[string]types.Type{"label": types.StringType()},
			sql:       "SELECT @label",
			wantParam: observedParam{TypeKind: generated.TypeKind_TYPE_STRING},
		},
		{
			name:    "undeclared @nope rejected under strict mode",
			sql:     "SELECT @nope",
			wantErr: &AnalyzeError{},
		},
		{
			name:    "@id declared but @other referenced",
			params:  map[string]types.Type{"id": types.Int64Type()},
			sql:     "SELECT @other",
			wantErr: &AnalyzeError{},
		},
		{
			name:                      "AllowUndeclaredParameters resolves @nope as untyped INT64",
			allowUndeclaredParameters: true,
			sql:                       "SELECT @nope",
			// Analyzer assigns the default INT64 to undeclared params
			// in permissive mode but flags IsUntyped so callers can
			// tell it apart from a declared INT64 (asserted on the
			// next case as well).
			wantParam: observedParam{TypeKind: generated.TypeKind_TYPE_INT64, IsUntyped: true},
		},
		{
			name:                      "AllowUndeclaredParameters resolves @label as untyped INT64",
			allowUndeclaredParameters: true,
			sql:                       "SELECT @label",
			wantParam:                 observedParam{TypeKind: generated.TypeKind_TYPE_INT64, IsUntyped: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := t.Context()
			a := newTestEngine(t)
			opts := newQueryStmtAnalyzerOptions()
			opts.ParameterMode = ParameterNamed
			opts.QueryParameters = tt.params
			opts.AllowUndeclaredParameters = tt.allowUndeclaredParameters

			// Act
			got, err := a.Analyze(ctx, tt.sql, nil, opts)

			// Assert
			if tt.wantErr != nil {
				assert.IsType(t, tt.wantErr, err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			stmt, ok := got.Statement.(*resolved_ast.QueryStmtNode)
			require.True(t, ok, "Statement is %T, want *resolved_ast.QueryStmtNode", got.Statement)
			param := findNode[*resolved_ast.ParameterNode](t, stmt)
			assert.Equal(t, tt.wantParam, observeParam(param))
		})
	}
}

// TestAnalyzerOptions_Clone_doesNotShareQueryParameters verifies that
// the Clone method's QueryParameters map is independent — mutating the
// clone's map must not leak into the original.
func TestAnalyzerOptions_Clone_doesNotShareQueryParameters(t *testing.T) {
	// Arrange
	sut := &AnalyzerOptions{
		QueryParameters: map[string]types.Type{
			"id": types.Int64Type(),
		},
	}

	// Act
	clone := sut.Clone()
	clone.QueryParameters["new"] = types.StringType()
	got := sut.QueryParameters

	// Assert
	want := map[string]types.Type{"id": types.Int64Type()}
	assert.Equal(t, want, got)
}
