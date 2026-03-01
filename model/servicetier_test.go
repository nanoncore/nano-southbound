package model

import "testing"

func intPtr(v int) *int  { return &v }
func boolP(v bool) *bool { return &v }

func TestServiceTierGetPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority *int
		expected int
	}{
		{
			name:     "nil Priority defaults to 3",
			priority: nil,
			expected: 3,
		},
		{
			name:     "Priority set to 5",
			priority: intPtr(5),
			expected: 5,
		},
		{
			name:     "Priority set to 0",
			priority: intPtr(0),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &ServiceTier{
				Name: "test-tier",
				Spec: ServiceTierSpec{
					BandwidthUp:   100,
					BandwidthDown: 200,
					Priority:      tt.priority,
				},
			}

			got := tier.GetPriority()
			if got != tt.expected {
				t.Errorf("GetPriority() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestServiceTierIsIPv6Enabled(t *testing.T) {
	tests := []struct {
		name        string
		ipv6Enabled *bool
		expected    bool
	}{
		{
			name:        "nil IPv6Enabled defaults to true",
			ipv6Enabled: nil,
			expected:    true,
		},
		{
			name:        "IPv6Enabled is true",
			ipv6Enabled: boolP(true),
			expected:    true,
		},
		{
			name:        "IPv6Enabled is false",
			ipv6Enabled: boolP(false),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &ServiceTier{
				Name: "test-tier",
				Spec: ServiceTierSpec{
					BandwidthUp:   100,
					BandwidthDown: 200,
					IPv6Enabled:   tt.ipv6Enabled,
				},
			}

			got := tier.IsIPv6Enabled()
			if got != tt.expected {
				t.Errorf("IsIPv6Enabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServiceTierGetBurstUp(t *testing.T) {
	tests := []struct {
		name     string
		burstUp  *int
		expected int
	}{
		{
			name:     "nil BurstUp returns 0",
			burstUp:  nil,
			expected: 0,
		},
		{
			name:     "BurstUp 10 MB converts to bytes",
			burstUp:  intPtr(10),
			expected: 10 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &ServiceTier{
				Name: "test-tier",
				Spec: ServiceTierSpec{
					BandwidthUp:   100,
					BandwidthDown: 200,
					BurstUp:       tt.burstUp,
				},
			}

			got := tier.GetBurstUp()
			if got != tt.expected {
				t.Errorf("GetBurstUp() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestServiceTierGetBurstDown(t *testing.T) {
	tests := []struct {
		name      string
		burstDown *int
		expected  int
	}{
		{
			name:      "nil BurstDown returns 0",
			burstDown: nil,
			expected:  0,
		},
		{
			name:      "BurstDown 5 MB converts to bytes",
			burstDown: intPtr(5),
			expected:  5 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := &ServiceTier{
				Name: "test-tier",
				Spec: ServiceTierSpec{
					BandwidthUp:   100,
					BandwidthDown: 200,
					BurstDown:     tt.burstDown,
				},
			}

			got := tier.GetBurstDown()
			if got != tt.expected {
				t.Errorf("GetBurstDown() = %d, want %d", got, tt.expected)
			}
		})
	}
}
