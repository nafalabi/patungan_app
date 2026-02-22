package services

import (
	"testing"
)

func TestNormalizeChatID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "phone number without country code",
			input:    "081246361829",
			expected: "6281246361829@c.us",
		},
		{
			name:     "phone number with country code",
			input:    "6281246361829",
			expected: "6281246361829@c.us",
		},
		{
			name:     "group id",
			input:    "120363407813232111@g.us",
			expected: "120363407813232111@g.us",
		},
		{
			name:     "phone number without country code, with suffix",
			input:    "081246361829@c.us",
			expected: "6281246361829@c.us",
		},
		{
			name:     "phone number with country code, with suffix",
			input:    "6281246361829@c.us",
			expected: "6281246361829@c.us",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeChatID(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeChatID(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
