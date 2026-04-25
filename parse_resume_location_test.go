package zetasql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewParseResumeLocation(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want *ParseResumeLocation
	}{
		{
			name: "non-empty SQL",
			sql:  "SELECT 1; SELECT 2",
			want: &ParseResumeLocation{Input: "SELECT 1; SELECT 2", BytePosition: 0},
		},
		{
			name: "single statement",
			sql:  "SELECT 1",
			want: &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := NewParseResumeLocation

			// Act
			got := sut(tt.sql)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseResumeLocation_Reset(t *testing.T) {
	tests := []struct {
		name    string
		initial *ParseResumeLocation
		want    *ParseResumeLocation
	}{
		{
			name:    "Reset from non-zero position",
			initial: &ParseResumeLocation{Input: "SELECT 1; SELECT 2", BytePosition: 10},
			want:    &ParseResumeLocation{Input: "SELECT 1; SELECT 2", BytePosition: 0},
		},
		{
			name:    "Reset already at zero",
			initial: &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
			want:    &ParseResumeLocation{Input: "SELECT 1", BytePosition: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sut := tt.initial

			// Act
			sut.Reset()
			got := sut

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
