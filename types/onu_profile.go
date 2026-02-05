package types

import (
	"context"
	"fmt"
)

// ONUProfileManager defines CRUD operations for ONU hardware profiles.
// Implemented by vendors that support profile management via CLI.
type ONUProfileManager interface {
	ListONUProfiles(ctx context.Context) ([]*ONUHardwareProfile, error)
	GetONUProfile(ctx context.Context, name string) (*ONUHardwareProfile, error)
	CreateONUProfile(ctx context.Context, profile *ONUHardwareProfile) error
	DeleteONUProfile(ctx context.Context, name string) error
}

// ONUProfilePorts captures port count settings for an ONU hardware profile.
type ONUProfilePorts struct {
	Eth      *int `json:"eth,omitempty"`
	Pots     *int `json:"pots,omitempty"`
	IPHost   *int `json:"iphost,omitempty"`
	IPv6Host *int `json:"ipv6host,omitempty"`
	Veip     *int `json:"veip,omitempty"`
}

// ONUHardwareProfile defines the hardware capabilities for an ONU profile
// (V-SOL: "profile onu").
type ONUHardwareProfile struct {
	// Name is the profile name (required for create/delete/show).
	Name string `json:"name"`

	// ID is the OLT-assigned profile ID (optional, set on read/list).
	ID *int `json:"id,omitempty"`

	// Description is a human-readable profile description.
	// V-SOL limits this to 64 characters.
	Description *string `json:"description,omitempty"`

	// Ports defines the number of ports by type.
	Ports *ONUProfilePorts `json:"ports,omitempty"`

	// TcontNum is the maximum TCONT count (1-255).
	TcontNum *int `json:"tcont_num,omitempty"`

	// GemportNum is the maximum GEMPORT count (1-255).
	GemportNum *int `json:"gemport_num,omitempty"`

	// SwitchNum is the ONU switch number.
	SwitchNum *int `json:"switch_num,omitempty"`

	// ServiceAbility defines the service ability (e.g., "n:1").
	ServiceAbility *string `json:"service_ability,omitempty"`

	// OmciSendMode is the OMCI send mode (vendor-specific).
	OmciSendMode *string `json:"omci_send_mode,omitempty"`

	// ExOMCI enables extended OMCI behavior.
	ExOMCI *bool `json:"ex_omci,omitempty"`

	// WifiMngViaNonOMCI controls WiFi management via non-OMCI.
	WifiMngViaNonOMCI *bool `json:"wifi_mng_via_non_omci,omitempty"`

	// DefaultMulticastRange configures the default multicast range (vendor-specific).
	DefaultMulticastRange *string `json:"default_multicast_range,omitempty"`

	// Committed indicates whether the profile is committed on the OLT.
	Committed *bool `json:"committed,omitempty"`
}

// Validate checks that the ONU hardware profile parameters are valid.
func (p *ONUHardwareProfile) Validate() error {
	if p == nil {
		return fmt.Errorf("profile is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.Description != nil && len(*p.Description) > 64 {
		return fmt.Errorf("description exceeds 64 characters")
	}

	if p.Ports != nil {
		if err := validateRangePtr("port-num eth", p.Ports.Eth, 1, 255); err != nil {
			return err
		}
		if err := validateRangePtr("port-num pots", p.Ports.Pots, 1, 255); err != nil {
			return err
		}
		if err := validateRangePtr("port-num iphost", p.Ports.IPHost, 1, 255); err != nil {
			return err
		}
		if err := validateRangePtr("port-num ipv6host", p.Ports.IPv6Host, 1, 255); err != nil {
			return err
		}
		if err := validateRangePtr("port-num veip", p.Ports.Veip, 1, 255); err != nil {
			return err
		}
	}

	if err := validateRangePtr("tcont-num", p.TcontNum, 1, 255); err != nil {
		return err
	}
	if err := validateRangePtr("gemport-num", p.GemportNum, 1, 255); err != nil {
		return err
	}
	if err := validateRangePtr("switch-num", p.SwitchNum, 1, 255); err != nil {
		return err
	}

	if p.TcontNum != nil && p.GemportNum == nil {
		return fmt.Errorf("tcont-num requires gemport-num")
	}
	if p.GemportNum != nil && p.TcontNum == nil {
		return fmt.Errorf("gemport-num requires tcont-num")
	}

	return nil
}

func validateRangePtr(field string, value *int, min, max int) error {
	if value == nil {
		return nil
	}
	if *value < min || *value > max {
		return fmt.Errorf("%s must be between %d and %d", field, min, max)
	}
	return nil
}
