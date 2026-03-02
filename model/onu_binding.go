package model

// ONUBindingRole defines the role of an ONU binding within a subscriber.
type ONUBindingRole string

const (
	// ONUBindingRolePrimary is the main ONU for the subscriber.
	ONUBindingRolePrimary ONUBindingRole = "primary"

	// ONUBindingRoleSecondary is an additional ONU for the subscriber.
	ONUBindingRoleSecondary ONUBindingRole = "secondary"

	// ONUBindingRoleRedundant is a backup ONU for failover.
	ONUBindingRoleRedundant ONUBindingRole = "redundant"
)

// ONUBinding represents the association between a subscriber and a physical ONU.
// A subscriber may have multiple ONUBindings for multi-ONU provisioning (1:N).
type ONUBinding struct {
	// Serial is the ONU serial number (e.g., "VSOL12345678")
	Serial string `json:"serial"`

	// PONPort is the PON port where this ONU is connected (e.g., "0/1")
	PONPort string `json:"ponPort"`

	// ONUID is the ONU ID assigned on the PON port
	ONUID int `json:"onuId"`

	// Role defines whether this is the primary, secondary, or redundant ONU
	Role ONUBindingRole `json:"role"`

	// Status is the current provisioning status of this binding
	Status string `json:"status,omitempty"`

	// Metadata contains additional vendor-specific or operational data
	Metadata map[string]string `json:"metadata,omitempty"`
}
