package types

import "testing"

func TestIsPowerWithinSpec(t *testing.T) {
	tests := []struct {
		name   string
		rxDBm  float64
		txDBm  float64
		expect bool
	}{
		{
			name:   "both within spec",
			rxDBm:  -15.0,
			txDBm:  2.5,
			expect: true,
		},
		{
			name:   "rx too low",
			rxDBm:  -30.0,
			txDBm:  2.5,
			expect: false,
		},
		{
			name:   "rx too high",
			rxDBm:  -5.0,
			txDBm:  2.5,
			expect: false,
		},
		{
			name:   "tx too low",
			rxDBm:  -15.0,
			txDBm:  0.2,
			expect: false,
		},
		{
			name:   "tx too high",
			rxDBm:  -15.0,
			txDBm:  6.0,
			expect: false,
		},
		{
			name:   "both out of spec",
			rxDBm:  -30.0,
			txDBm:  0.2,
			expect: false,
		},
		{
			name:   "edge case rx at low boundary tx at low boundary",
			rxDBm:  -28.0,
			txDBm:  0.5,
			expect: true,
		},
		{
			name:   "edge case rx at high boundary tx at high boundary",
			rxDBm:  -8.0,
			txDBm:  5.0,
			expect: true,
		},
		{
			name:   "rx just below low threshold",
			rxDBm:  -28.01,
			txDBm:  2.5,
			expect: false,
		},
		{
			name:   "rx just above high threshold",
			rxDBm:  -7.99,
			txDBm:  2.5,
			expect: false,
		},
		{
			name:   "tx just below low threshold",
			rxDBm:  -15.0,
			txDBm:  0.49,
			expect: false,
		},
		{
			name:   "tx just above high threshold",
			rxDBm:  -15.0,
			txDBm:  5.01,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPowerWithinSpec(tt.rxDBm, tt.txDBm)
			if got != tt.expect {
				t.Errorf("IsPowerWithinSpec(%v, %v) = %v, want %v", tt.rxDBm, tt.txDBm, got, tt.expect)
			}
		})
	}
}

func TestHumanErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *HumanError
		expected string
	}{
		{
			name: "without action",
			err: &HumanError{
				Code:    ErrCodeONUNotFound,
				Message: "ONU not found on PON port 0/1",
				Vendor:  "vsol",
			},
			expected: "[vsol] ONU_NOT_FOUND: ONU not found on PON port 0/1",
		},
		{
			name: "with action",
			err: &HumanError{
				Code:    ErrCodeTimeout,
				Message: "Connection timed out after 30s",
				Action:  "Check if the OLT is reachable",
				Vendor:  "huawei",
			},
			expected: "[huawei] TIMEOUT: Connection timed out after 30s (Suggestion: Check if the OLT is reachable)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHumanErrorIsRecoverable(t *testing.T) {
	tests := []struct {
		name        string
		recoverable bool
		expected    bool
	}{
		{
			name:        "recoverable true",
			recoverable: true,
			expected:    true,
		},
		{
			name:        "recoverable false",
			recoverable: false,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &HumanError{
				Code:        ErrCodeTimeout,
				Message:     "test",
				Vendor:      "test",
				Recoverable: tt.recoverable,
			}
			got := err.IsRecoverable()
			if got != tt.expected {
				t.Errorf("IsRecoverable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCodeConstantsNonEmpty(t *testing.T) {
	codes := []struct {
		name  string
		value string
	}{
		{"ErrCodeONUExists", ErrCodeONUExists},
		{"ErrCodeONUNotFound", ErrCodeONUNotFound},
		{"ErrCodeInvalidSerial", ErrCodeInvalidSerial},
		{"ErrCodePortNotFound", ErrCodePortNotFound},
		{"ErrCodeConfigLocked", ErrCodeConfigLocked},
		{"ErrCodeTimeout", ErrCodeTimeout},
		{"ErrCodeConnReset", ErrCodeConnReset},
		{"ErrCodeONUFull", ErrCodeONUFull},
		{"ErrCodeProfileNotFound", ErrCodeProfileNotFound},
		{"ErrCodeUnknownCommand", ErrCodeUnknownCommand},
		{"ErrCodeAuthFailed", ErrCodeAuthFailed},
		{"ErrCodeUnknown", ErrCodeUnknown},
	}

	for _, c := range codes {
		t.Run(c.name, func(t *testing.T) {
			if c.value == "" {
				t.Errorf("%s should not be empty", c.name)
			}
		})
	}
}

func TestGPONThresholdConstants(t *testing.T) {
	// Rx thresholds: low should be less than high
	if GPONRxLowThreshold >= GPONRxHighThreshold {
		t.Errorf("GPONRxLowThreshold (%v) should be less than GPONRxHighThreshold (%v)",
			GPONRxLowThreshold, GPONRxHighThreshold)
	}

	// Tx thresholds: low should be less than high
	if GPONTxLowThreshold >= GPONTxHighThreshold {
		t.Errorf("GPONTxLowThreshold (%v) should be less than GPONTxHighThreshold (%v)",
			GPONTxLowThreshold, GPONTxHighThreshold)
	}

	// Rx values should be negative (dBm for fiber)
	if GPONRxLowThreshold >= 0 {
		t.Errorf("GPONRxLowThreshold (%v) should be negative", GPONRxLowThreshold)
	}
	if GPONRxHighThreshold >= 0 {
		t.Errorf("GPONRxHighThreshold (%v) should be negative", GPONRxHighThreshold)
	}

	// Tx values should be positive (transmit power)
	if GPONTxLowThreshold <= 0 {
		t.Errorf("GPONTxLowThreshold (%v) should be positive", GPONTxLowThreshold)
	}
	if GPONTxHighThreshold <= 0 {
		t.Errorf("GPONTxHighThreshold (%v) should be positive", GPONTxHighThreshold)
	}
}
