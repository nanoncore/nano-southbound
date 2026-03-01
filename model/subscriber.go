// Package model contains domain types for subscriber and service tier provisioning.
// These are lightweight versions of the Kubernetes CRD types, suitable for use
// in the southbound drivers without requiring k8s.io dependencies.
package model

// Subscriber represents a subscriber to be provisioned on network equipment.
// This is a simplified version of the Kubernetes Subscriber CRD.
type Subscriber struct {
	// Name is the unique identifier for this subscriber
	Name string

	// Annotations contains additional metadata
	Annotations map[string]string

	// Spec contains the subscriber specification
	Spec SubscriberSpec
}

// SubscriberSpec defines the desired state of a Subscriber
type SubscriberSpec struct {
	// ONUSerial is the ONU serial number (ONT/CPE identifier)
	// Format: 4 letters + 8 hex digits (e.g., VSOL12345678)
	ONUSerial string `json:"onuSerial"`

	// MACAddress is the MAC address of the CPE (for EPON)
	MACAddress string `json:"macAddress,omitempty"`

	// VLAN is the subscriber VLAN ID (C-VLAN in Q-in-Q)
	VLAN int `json:"vlan"`

	// SVLAN is the service VLAN ID (S-VLAN in Q-in-Q, optional)
	SVLAN *int `json:"svlan,omitempty"`

	// IPAddress is a static IPv4 address (optional, otherwise DHCP)
	IPAddress string `json:"ipAddress,omitempty"`

	// IPv6Address is a static IPv6 address (optional, otherwise DHCPv6/SLAAC)
	IPv6Address string `json:"ipv6Address,omitempty"`

	// Tier is the name of the ServiceTier
	Tier string `json:"tier"`

	// Username is the PPPoE/IPoE username for authentication
	Username string `json:"username,omitempty"`

	// Password is the PPPoE/IPoE password
	Password string `json:"password,omitempty"`

	// Enabled controls whether the subscriber has access
	Enabled *bool `json:"enabled,omitempty"`

	// DelegatedPrefix is the IPv6 prefix delegation (e.g., /56, /60)
	DelegatedPrefix string `json:"delegatedPrefix,omitempty"`

	// CircuitID is the DHCP Option 82 Circuit ID
	CircuitID string `json:"circuitId,omitempty"`

	// RemoteID is the DHCP Option 82 Remote ID
	RemoteID string `json:"remoteId,omitempty"`

	// Description is a human-readable description
	Description string `json:"description,omitempty"`

	// ONUBindings holds the 1:N subscriber-to-ONU associations.
	// For new subscribers, use this field instead of ONUSerial.
	// For backward compatibility, if ONUBindings is empty the legacy
	// ONUSerial field is used to synthesize a primary binding.
	ONUBindings []ONUBinding `json:"onuBindings,omitempty"`
}

// IsEnabled returns true if the subscriber is enabled (default: true)
func (s *Subscriber) IsEnabled() bool {
	if s.Spec.Enabled == nil {
		return true
	}
	return *s.Spec.Enabled
}

// GetPrimaryONU returns the primary ONUBinding for this subscriber.
// If ONUBindings is populated, it returns the first binding with Role "primary".
// If ONUBindings is empty but the legacy ONUSerial field is set, it synthesizes
// a primary ONUBinding from that field for backward compatibility.
// Returns nil if no ONU binding can be determined.
func (s *Subscriber) GetPrimaryONU() *ONUBinding {
	for i := range s.Spec.ONUBindings {
		if s.Spec.ONUBindings[i].Role == ONUBindingRolePrimary {
			return &s.Spec.ONUBindings[i]
		}
	}
	// Backward compatibility: synthesize from legacy ONUSerial
	if s.Spec.ONUSerial != "" {
		return &ONUBinding{
			Serial: s.Spec.ONUSerial,
			Role:   ONUBindingRolePrimary,
		}
	}
	return nil
}

// GetONUBySerial finds an ONUBinding by its serial number.
// Returns nil if not found. Also checks the legacy ONUSerial field.
func (s *Subscriber) GetONUBySerial(serial string) *ONUBinding {
	for i := range s.Spec.ONUBindings {
		if s.Spec.ONUBindings[i].Serial == serial {
			return &s.Spec.ONUBindings[i]
		}
	}
	// Backward compatibility: check legacy field
	if len(s.Spec.ONUBindings) == 0 && s.Spec.ONUSerial == serial {
		return &ONUBinding{
			Serial: s.Spec.ONUSerial,
			Role:   ONUBindingRolePrimary,
		}
	}
	return nil
}

// AddONU appends an ONUBinding to the subscriber.
func (s *Subscriber) AddONU(binding ONUBinding) {
	s.Spec.ONUBindings = append(s.Spec.ONUBindings, binding)
}

// RemoveONU removes an ONUBinding by serial number.
// Returns true if a binding was removed, false if not found.
func (s *Subscriber) RemoveONU(serial string) bool {
	for i, b := range s.Spec.ONUBindings {
		if b.Serial == serial {
			s.Spec.ONUBindings = append(s.Spec.ONUBindings[:i], s.Spec.ONUBindings[i+1:]...)
			return true
		}
	}
	return false
}
