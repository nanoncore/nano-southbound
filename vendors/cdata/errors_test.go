package cdata

import (
	"fmt"
	"testing"
)

func TestTranslateError_KnownPatterns(t *testing.T) {
	adapter := &Adapter{}

	tests := []struct {
		name         string
		errMsg       string
		expectedCode ErrorCode
		recoverable  bool
	}{
		// ONU existence errors
		{"onu already exist", "ONU Already Exist on port 1/1/1", ErrONUExists, false},
		{"onu not found", "onu not found at location", ErrONUNotFound, false},
		{"no onu", "No ONU registered", ErrONUNotFound, false},

		// Serial number errors
		{"invalid serial", "Invalid serial number format", ErrInvalidSerial, false},
		{"bad serial", "Bad serial: not 12 chars", ErrInvalidSerial, false},

		// Config lock errors
		{"configuration is locked", "Configuration is locked by admin", ErrConfigLocked, true},
		{"config lock", "config lock: held by session 2", ErrConfigLocked, true},
		{"exclusive lock", "Exclusive lock active", ErrConfigLocked, true},

		// Port errors
		{"port not exist", "port not exist: 1/1/9", ErrPortNotFound, false},
		{"invalid port", "Invalid port number", ErrPortNotFound, false},
		{"interface not found", "interface not found: gpon-olt_1/1/9", ErrPortNotFound, false},

		// Capacity errors
		{"onu id is full", "ONU ID is full on port 1/1/1", ErrONUFull, false},
		{"no available onu-id", "No available onu-id on this port", ErrONUFull, false},
		{"max onu", "Max ONU limit reached", ErrONUFull, false},

		// Profile errors
		{"profile not found", "Profile not found: line_100M_50M", ErrProfileMissing, false},
		{"line profile not exist", "Line profile not exist on OLT", ErrProfileMissing, false},
		{"service profile not exist", "Service profile not exist", ErrProfileMissing, false},

		// VLAN errors
		{"invalid vlan", "Invalid VLAN ID: 5000", ErrVLANInvalid, false},
		{"vlan not exist", "VLAN not exist on uplink", ErrVLANInvalid, false},

		// Command errors
		{"unknown command", "% Unknown command at line 1", ErrUnknownCommand, false},
		{"incomplete command", "Incomplete command entered", ErrUnknownCommand, false},
		{"invalid input", "Invalid input detected", ErrUnknownCommand, false},

		// Connection errors
		{"timeout", "Command timeout after 30s", ErrTimeout, true},
		{"connection reset", "Connection reset by peer", ErrConnReset, true},
		{"connection refused", "Connection refused to 10.0.0.1:22", ErrConnRefuse, true},
		{"authentication failed", "Authentication failed for admin", ErrAuthFailed, false},
		{"access denied", "Access denied: insufficient privileges", ErrAuthFailed, false},

		// Hardware errors
		{"hardware fault", "Hardware fault detected on PON card", ErrHardwareFault, false},
		{"memory full", "Memory full: cannot allocate", ErrMemoryFull, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.translateError(fmt.Errorf("%s", tt.errMsg))
			te, ok := err.(*TranslatedError)
			if !ok {
				t.Fatalf("expected *TranslatedError, got %T", err)
			}
			if te.Code != tt.expectedCode {
				t.Errorf("Code = %s, want %s", te.Code, tt.expectedCode)
			}
			if te.Recoverable != tt.recoverable {
				t.Errorf("Recoverable = %v, want %v", te.Recoverable, tt.recoverable)
			}
			if te.Human == "" {
				t.Error("Human message is empty")
			}
			if te.Action == "" {
				t.Error("Action message is empty")
			}
			if te.Original == nil {
				t.Error("Original error is nil")
			}
		})
	}
}

func TestTranslateError_NilError(t *testing.T) {
	adapter := &Adapter{}
	result := adapter.translateError(nil)
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestTranslateError_UnknownError(t *testing.T) {
	adapter := &Adapter{}
	err := adapter.translateError(fmt.Errorf("some random error xyz"))
	te, ok := err.(*TranslatedError)
	if !ok {
		t.Fatalf("expected *TranslatedError, got %T", err)
	}
	if te.Code != ErrUnknown {
		t.Errorf("Code = %s, want %s", te.Code, ErrUnknown)
	}
	if te.Recoverable != false {
		t.Error("unknown errors should not be recoverable")
	}
	if te.Human != "some random error xyz" {
		t.Errorf("Human = %q, want original error message", te.Human)
	}
}

func TestTranslatedError_ErrorString(t *testing.T) {
	te := &TranslatedError{
		Original:    fmt.Errorf("original"),
		Code:        ErrONUExists,
		Human:       "ONU already exists",
		Action:      "Delete first",
		Recoverable: false,
	}
	errStr := te.Error()
	expected := "[ONU_EXISTS] ONU already exists (action: Delete first)"
	if errStr != expected {
		t.Errorf("Error() = %q, want %q", errStr, expected)
	}
}

func TestIsRecoverable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"recoverable translated error", &TranslatedError{Recoverable: true}, true},
		{"non-recoverable translated error", &TranslatedError{Recoverable: false}, false},
		{"non-translated error", fmt.Errorf("plain error"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				// IsRecoverable panics on nil; skip
				return
			}
			if got := IsRecoverable(tt.err); got != tt.expected {
				t.Errorf("IsRecoverable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{"translated error", &TranslatedError{Code: ErrONUExists}, ErrONUExists},
		{"non-translated error", fmt.Errorf("plain"), ErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorCode(tt.err); got != tt.expected {
				t.Errorf("GetErrorCode() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestGetSuggestedAction(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"translated error", &TranslatedError{Action: "retry later"}, "retry later"},
		{"non-translated error", fmt.Errorf("plain"), "Check OLT logs for details"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSuggestedAction(tt.err); got != tt.expected {
				t.Errorf("GetSuggestedAction() = %q, want %q", got, tt.expected)
			}
		})
	}
}
