package state

import (
	"testing"
	"time"
)

func TestGetLogicalDateUTC8(t *testing.T) {
	locUTC8 := time.FixedZone("UTC+8", 8*3600)

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "Right before 4:00 AM UTC+8 belongs to previous day",
			input:    time.Date(2026, 8, 28, 3, 59, 59, 0, locUTC8),
			expected: "2026-08-27",
		},
		{
			name:     "Exactly at 4:00 AM UTC+8 belongs to current day",
			input:    time.Date(2026, 8, 28, 4, 0, 0, 0, locUTC8),
			expected: "2026-08-28",
		},
		{
			name:     "After 4:00 AM UTC+8 belongs to current day",
			input:    time.Date(2026, 8, 28, 15, 30, 0, 0, locUTC8),
			expected: "2026-08-28",
		},
		{
			name:     "Midnight UTC belongs to 8:00 AM UTC+8 (current day)",
			input:    time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), // 8:00 AM UTC+8
			expected: "2026-08-28",
		},
		{
			name:     "19:00 UTC previous day belongs to 03:00 UTC+8 (previous day)",
			input:    time.Date(2026, 8, 27, 19, 0, 0, 0, time.UTC), // 03:00 AM 2026-08-28 UTC+8
			expected: "2026-08-27",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLogicalDateUTC8(tt.input)
			if got != tt.expected {
				t.Errorf("GetLogicalDateUTC8() = %v, expected %v", got, tt.expected)
			}
		})
	}
}
