package zetasql

import (
	"testing"

	"github.com/glassmonkey/zetasql-wasm/wasm/generated"
)

func TestAnalyzerOptions_SetParseLocationRecordType(t *testing.T) {
	opts := NewAnalyzerOptions()
	opts.SetParseLocationRecordType(generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE)

	proto := opts.toProto()
	if proto.ParseLocationRecordType == nil {
		t.Fatal("ParseLocationRecordType = nil")
	}
	if got, want := *proto.ParseLocationRecordType, generated.ParseLocationRecordType_PARSE_LOCATION_RECORD_FULL_NODE_SCOPE; got != want {
		t.Errorf("ParseLocationRecordType = %v, want %v", got, want)
	}
}

func TestAnalyzerOptions_DefaultNilFields(t *testing.T) {
	opts := NewAnalyzerOptions()
	proto := opts.toProto()

	if proto.LanguageOptions != nil {
		t.Errorf("LanguageOptions = %v, want nil", proto.LanguageOptions)
	}
	if proto.ParseLocationRecordType != nil {
		t.Errorf("ParseLocationRecordType = %v, want nil", proto.ParseLocationRecordType)
	}
}
