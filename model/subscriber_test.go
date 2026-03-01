package model

import "testing"

func boolPtr(v bool) *bool { return &v }

func TestSubscriberIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  *bool
		expected bool
	}{
		{
			name:     "nil Enabled defaults to true",
			enabled:  nil,
			expected: true,
		},
		{
			name:     "Enabled is true",
			enabled:  boolPtr(true),
			expected: true,
		},
		{
			name:     "Enabled is false",
			enabled:  boolPtr(false),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscriber{
				Name: "test-sub",
				Spec: SubscriberSpec{
					ONUSerial: "VSOL12345678",
					VLAN:      100,
					Tier:      "default",
					Enabled:   tt.enabled,
				},
			}

			got := sub.IsEnabled()
			if got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}
