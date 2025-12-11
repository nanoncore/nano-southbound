package model

// ServiceTier represents a service tier defining bandwidth and QoS parameters.
// This is a simplified version of the Kubernetes ServiceTier CRD.
type ServiceTier struct {
	// Name is the unique identifier for this tier
	Name string

	// Annotations contains additional metadata
	Annotations map[string]string

	// Spec contains the tier specification
	Spec ServiceTierSpec
}

// ServiceTierSpec defines the parameters for a service tier
type ServiceTierSpec struct {
	// BandwidthUp is the upload bandwidth in Mbps
	BandwidthUp int `json:"bandwidthUp"`

	// BandwidthDown is the download bandwidth in Mbps
	BandwidthDown int `json:"bandwidthDown"`

	// QoSClass defines the quality of service class
	// Values: best-effort, standard, premium, business
	QoSClass string `json:"qosClass,omitempty"`

	// BurstUp is the upload burst size in MB (optional)
	BurstUp *int `json:"burstUp,omitempty"`

	// BurstDown is the download burst size in MB (optional)
	BurstDown *int `json:"burstDown,omitempty"`

	// Priority is the traffic priority (0-7, higher is better)
	Priority *int `json:"priority,omitempty"`

	// Description is a human-readable description
	Description string `json:"description,omitempty"`

	// IPv6Enabled enables IPv6 for this tier
	IPv6Enabled *bool `json:"ipv6Enabled,omitempty"`

	// StaticIPPool is an optional static IP pool CIDR
	StaticIPPool string `json:"staticIpPool,omitempty"`
}

// GetPriority returns the priority or default (3)
func (t *ServiceTier) GetPriority() int {
	if t.Spec.Priority == nil {
		return 3
	}
	return *t.Spec.Priority
}

// IsIPv6Enabled returns true if IPv6 is enabled (default: true)
func (t *ServiceTier) IsIPv6Enabled() bool {
	if t.Spec.IPv6Enabled == nil {
		return true
	}
	return *t.Spec.IPv6Enabled
}

// GetBurstUp returns burst up in bytes, or 0 if not set
func (t *ServiceTier) GetBurstUp() int {
	if t.Spec.BurstUp == nil {
		return 0
	}
	return *t.Spec.BurstUp * 1024 * 1024 // Convert MB to bytes
}

// GetBurstDown returns burst down in bytes, or 0 if not set
func (t *ServiceTier) GetBurstDown() int {
	if t.Spec.BurstDown == nil {
		return 0
	}
	return *t.Spec.BurstDown * 1024 * 1024 // Convert MB to bytes
}
