package huawei

import "testing"

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"int", int(1), 1},
		{"int_negative", int(-1), -1},
		{"int64", int64(2), 2},
		{"int64_large", int64(9223372036854775807), 9223372036854775807},
		{"uint", uint(3), 3},
		{"uint64", uint64(4), 4},
		{"string", "invalid", -1},
		{"nil", nil, -1},
		{"float64", float64(1.5), -1},
		{"bool", true, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toInt(tt.input); got != tt.expected {
				t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
