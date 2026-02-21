package types

import (
	"context"
	"time"
)

// WifiManager defines southbound Wi-Fi operations for vendor adapters.
type WifiManager interface {
	// GetWifiConfig reads ONU Wi-Fi configuration when supported.
	GetWifiConfig(ctx context.Context, target WifiTarget) (*WifiActionResult, error)

	// SetWifiConfig applies SSID/password/enabled on an ONU.
	SetWifiConfig(ctx context.Context, target WifiTarget, cfg WifiConfig) (*WifiActionResult, error)

	// SetWifiEnabled toggles ONU Wi-Fi state.
	SetWifiEnabled(ctx context.Context, target WifiTarget, enabled bool) (*WifiActionResult, error)
}

// WifiTarget identifies the ONU that Wi-Fi actions apply to.
type WifiTarget struct {
	// OnuSerial is the canonical identifier from northbound.
	OnuSerial string `json:"onuSerial,omitempty"`

	// Optional resolved coordinates when already known.
	PONPort string `json:"ponPort,omitempty"`
	ONUID   int    `json:"onuId,omitempty"`
}

// WifiConfig is the desired ONU Wi-Fi config.
type WifiConfig struct {
	SSID     string `json:"ssid,omitempty"`
	Password string `json:"password,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// WifiObservedSource defines the verification source for observed config.
type WifiObservedSource string

const (
	WifiObservedSourceOMCIReadback WifiObservedSource = "OMCI_READBACK"
	WifiObservedSourceACSVerify    WifiObservedSource = "ACS_VERIFY"
)

// WifiErrorCode is the normalized Wi-Fi action error code.
type WifiErrorCode string

const (
	WifiErrorCodeProfileNotOMCIReady WifiErrorCode = "PROFILE_NOT_OMCI_READY"
	WifiErrorCodeOnuOffline          WifiErrorCode = "ONU_OFFLINE"
	WifiErrorCodeOnuNotFound         WifiErrorCode = "ONU_NOT_FOUND"
	WifiErrorCodePartialApply        WifiErrorCode = "PARTIAL_APPLY"
	WifiErrorCodeCommandTimeout      WifiErrorCode = "COMMAND_TIMEOUT"
	WifiErrorCodeReadbackUnavailable WifiErrorCode = "READBACK_UNAVAILABLE"
	WifiErrorCodeCancelUnsupported   WifiErrorCode = "CANCEL_UNSUPPORTED"
	WifiErrorCodeRateLimited         WifiErrorCode = "RATE_LIMITED"
	WifiErrorCodeInvalidValue        WifiErrorCode = "INVALID_VALUE"
	WifiErrorCodePermissionDenied    WifiErrorCode = "PERMISSION_DENIED"
	WifiErrorCodeInternalError       WifiErrorCode = "INTERNAL_ERROR"
)

// WifiActionEvent captures per-step execution results.
type WifiActionEvent struct {
	Step      string    `json:"step"`
	OK        bool      `json:"ok"`
	Timestamp time.Time `json:"timestamp"`
	Detail    string    `json:"detail,omitempty"`
}

// WifiActionResult is the normalized output for Wi-Fi actions.
type WifiActionResult struct {
	OK bool `json:"ok"`

	ErrorCode WifiErrorCode `json:"errorCode,omitempty"`
	Reason    string        `json:"reason,omitempty"`

	RawOutput string `json:"rawOutput,omitempty"`

	ObservedConfig *WifiConfig         `json:"observedConfig,omitempty"`
	ObservedSource *WifiObservedSource `json:"observedSource,omitempty"`
	ObservedAt     *time.Time          `json:"observedAt,omitempty"`

	FailedStep string            `json:"failedStep,omitempty"`
	Events     []WifiActionEvent `json:"events,omitempty"`
}
