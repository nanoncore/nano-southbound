package cdata

import (
	"fmt"
	"strings"
)

// ErrorCode represents a normalized error code for C-Data errors
type ErrorCode string

const (
	// Configuration errors
	ErrONUExists      ErrorCode = "ONU_EXISTS"
	ErrONUNotFound    ErrorCode = "ONU_NOT_FOUND"
	ErrInvalidSerial  ErrorCode = "INVALID_SERIAL"
	ErrConfigLocked   ErrorCode = "CONFIG_LOCKED"
	ErrPortNotFound   ErrorCode = "PORT_NOT_FOUND"
	ErrONUFull        ErrorCode = "ONU_FULL"
	ErrProfileMissing ErrorCode = "PROFILE_MISSING"
	ErrVLANInvalid    ErrorCode = "VLAN_INVALID"
	ErrUnknownCommand ErrorCode = "UNKNOWN_CMD"

	// Connection errors
	ErrTimeout    ErrorCode = "TIMEOUT"
	ErrConnReset  ErrorCode = "CONN_RESET"
	ErrConnRefuse ErrorCode = "CONN_REFUSED"
	ErrAuthFailed ErrorCode = "AUTH_FAILED"

	// Hardware errors
	ErrHardwareFault ErrorCode = "HARDWARE_FAULT"
	ErrMemoryFull    ErrorCode = "MEMORY_FULL"

	// Unknown
	ErrUnknown ErrorCode = "UNKNOWN"
)

// ErrorMapping maps C-Data error patterns to human-readable messages
type ErrorMapping struct {
	Code        ErrorCode
	Human       string
	Action      string
	Recoverable bool
}

// cdataErrorPatterns maps C-Data CLI error strings to structured errors
var cdataErrorPatterns = map[string]ErrorMapping{
	// ONU existence errors
	"onu already exist": {
		Code:        ErrONUExists,
		Human:       "ONU is already registered on this OLT",
		Action:      "Delete existing ONU first or use update operation",
		Recoverable: false,
	},
	"onu not found": {
		Code:        ErrONUNotFound,
		Human:       "ONU is not registered",
		Action:      "Verify ONU serial and PON port",
		Recoverable: false,
	},
	"no onu": {
		Code:        ErrONUNotFound,
		Human:       "ONU does not exist at this location",
		Action:      "Check PON port and ONU ID",
		Recoverable: false,
	},

	// Serial number errors
	"invalid serial": {
		Code:        ErrInvalidSerial,
		Human:       "Serial number format is invalid",
		Action:      "Check format: CDAT + 8 hex characters (e.g., CDAT12345678)",
		Recoverable: false,
	},
	"bad serial": {
		Code:        ErrInvalidSerial,
		Human:       "Serial number is malformed",
		Action:      "Verify serial number matches ONU label",
		Recoverable: false,
	},

	// Configuration lock errors
	"configuration is locked": {
		Code:        ErrConfigLocked,
		Human:       "Another session has the configuration lock",
		Action:      "Will retry automatically when lock is released",
		Recoverable: true,
	},
	"config lock": {
		Code:        ErrConfigLocked,
		Human:       "Configuration is locked by another user",
		Action:      "Wait for other session to complete or force unlock",
		Recoverable: true,
	},
	"exclusive lock": {
		Code:        ErrConfigLocked,
		Human:       "Exclusive configuration lock is held",
		Action:      "Will retry in a few seconds",
		Recoverable: true,
	},

	// Port errors
	"port not exist": {
		Code:        ErrPortNotFound,
		Human:       "PON port does not exist",
		Action:      "Verify PON port number (format: slot/frame/port)",
		Recoverable: false,
	},
	"invalid port": {
		Code:        ErrPortNotFound,
		Human:       "Invalid PON port specified",
		Action:      "Check port exists on this OLT model",
		Recoverable: false,
	},
	"interface not found": {
		Code:        ErrPortNotFound,
		Human:       "Interface does not exist",
		Action:      "Verify interface name matches OLT configuration",
		Recoverable: false,
	},

	// Capacity errors
	"onu id is full": {
		Code:        ErrONUFull,
		Human:       "Maximum ONUs reached on this PON port",
		Action:      "Delete unused ONUs to free slots (max 128 per port)",
		Recoverable: false,
	},
	"no available onu-id": {
		Code:        ErrONUFull,
		Human:       "No free ONU IDs available on this port",
		Action:      "Remove inactive ONUs or use a different port",
		Recoverable: false,
	},
	"max onu": {
		Code:        ErrONUFull,
		Human:       "Maximum ONU limit reached",
		Action:      "Check OLT license for ONU limits",
		Recoverable: false,
	},

	// Profile errors
	"profile not found": {
		Code:        ErrProfileMissing,
		Human:       "Line or service profile does not exist",
		Action:      "Create the profile first or use an existing profile name",
		Recoverable: false,
	},
	"line profile not exist": {
		Code:        ErrProfileMissing,
		Human:       "Line profile is not configured on this OLT",
		Action:      "Configure line profile before provisioning ONUs",
		Recoverable: false,
	},
	"service profile not exist": {
		Code:        ErrProfileMissing,
		Human:       "Service profile is not configured",
		Action:      "Create service profile with required parameters",
		Recoverable: false,
	},

	// VLAN errors
	"invalid vlan": {
		Code:        ErrVLANInvalid,
		Human:       "VLAN ID is out of range",
		Action:      "Use VLAN ID between 1 and 4094",
		Recoverable: false,
	},
	"vlan not exist": {
		Code:        ErrVLANInvalid,
		Human:       "VLAN is not configured on uplink",
		Action:      "Configure VLAN on upstream port first",
		Recoverable: false,
	},

	// Command errors
	"% unknown command": {
		Code:        ErrUnknownCommand,
		Human:       "Command not supported by this firmware",
		Action:      "Check OLT firmware version - may need upgrade",
		Recoverable: false,
	},
	"incomplete command": {
		Code:        ErrUnknownCommand,
		Human:       "Command is incomplete",
		Action:      "Internal error - contact support",
		Recoverable: false,
	},
	"invalid input": {
		Code:        ErrUnknownCommand,
		Human:       "Invalid command syntax",
		Action:      "Check command parameters",
		Recoverable: false,
	},

	// Connection errors
	"timeout": {
		Code:        ErrTimeout,
		Human:       "Command timed out",
		Action:      "Will retry with longer timeout",
		Recoverable: true,
	},
	"connection reset": {
		Code:        ErrConnReset,
		Human:       "Lost connection to OLT",
		Action:      "Will reconnect and retry",
		Recoverable: true,
	},
	"connection refused": {
		Code:        ErrConnRefuse,
		Human:       "Connection to OLT refused",
		Action:      "Check OLT is reachable and SSH/Telnet is enabled",
		Recoverable: true,
	},
	"authentication failed": {
		Code:        ErrAuthFailed,
		Human:       "Authentication failed",
		Action:      "Check username and password",
		Recoverable: false,
	},
	"access denied": {
		Code:        ErrAuthFailed,
		Human:       "Access denied",
		Action:      "Verify user has admin privileges",
		Recoverable: false,
	},

	// Hardware errors
	"hardware fault": {
		Code:        ErrHardwareFault,
		Human:       "OLT hardware fault detected",
		Action:      "Check OLT hardware status and logs",
		Recoverable: false,
	},
	"memory full": {
		Code:        ErrMemoryFull,
		Human:       "OLT memory is full",
		Action:      "Reboot OLT or clear old configurations",
		Recoverable: false,
	},
}

// TranslatedError represents a user-friendly error
type TranslatedError struct {
	Original    error
	Code        ErrorCode
	Human       string
	Action      string
	Recoverable bool
}

func (e *TranslatedError) Error() string {
	return fmt.Sprintf("[%s] %s (action: %s)", e.Code, e.Human, e.Action)
}

// translateError converts a raw CLI error into a human-readable error
func (a *Adapter) translateError(err error) error {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	for pattern, mapping := range cdataErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return &TranslatedError{
				Original:    err,
				Code:        mapping.Code,
				Human:       mapping.Human,
				Action:      mapping.Action,
				Recoverable: mapping.Recoverable,
			}
		}
	}

	// Unknown error
	return &TranslatedError{
		Original:    err,
		Code:        ErrUnknown,
		Human:       err.Error(),
		Action:      "Check OLT logs for details",
		Recoverable: false,
	}
}

// IsRecoverable returns true if the error can be retried
func IsRecoverable(err error) bool {
	if te, ok := err.(*TranslatedError); ok {
		return te.Recoverable
	}
	return false
}

// GetErrorCode returns the error code for a translated error
func GetErrorCode(err error) ErrorCode {
	if te, ok := err.(*TranslatedError); ok {
		return te.Code
	}
	return ErrUnknown
}

// GetSuggestedAction returns the suggested action for an error
func GetSuggestedAction(err error) string {
	if te, ok := err.(*TranslatedError); ok {
		return te.Action
	}
	return "Check OLT logs for details"
}
