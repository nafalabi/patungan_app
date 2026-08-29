package utils

import (
	"testing"
)

func TestSafeInitial(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback []string
		expected string
	}{
		{"normal name", "Netflix", nil, "N"},
		{"lowercase", "spotify", nil, "S"},
		{"empty string", "", nil, "—"},
		{"spaces only", "   ", nil, "—"},
		{"empty with fallback", "", []string{"P"}, "P"},
		{"spaces with fallback", "  ", []string{"U"}, "U"},
		{"unicode", "Über", nil, "Ü"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeInitial(tt.input, tt.fallback...)
			if got != tt.expected {
				t.Errorf("SafeInitial(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSafeInitials(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"full name", "John Doe", "JD"},
		{"double space", "John  Doe", "JD"},
		{"triple words", "John Middle Doe", "JD"},
		{"single word", "John", "JO"},
		{"single letter", "J", "J"},
		{"empty string", "", "—"},
		{"whitespace only", "   \t\n ", "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeInitials(tt.input)
			if got != tt.expected {
				t.Errorf("SafeInitials(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
