package types

import "time"

// VLANInfo represents a VLAN configured on the OLT.
type VLANInfo struct {
	// ID is the VLAN ID (1-4094)
	ID int `json:"id"`

	// Name is the VLAN name
	Name string `json:"name"`

	// Type is the VLAN type (e.g., "smart", "standard")
	Type string `json:"type"`

	// Description is the VLAN description
	Description string `json:"description,omitempty"`

	// ServicePortCount is the number of service ports using this VLAN
	ServicePortCount int `json:"service_port_count"`

	// CreatedAt is when the VLAN was created
	CreatedAt time.Time `json:"created_at,omitempty"`

	// Metadata contains vendor-specific VLAN data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ServicePort represents a service port mapping between a VLAN and an ONU.
type ServicePort struct {
	// Index is the service port index
	Index int `json:"index"`

	// VLAN is the service VLAN ID
	VLAN int `json:"vlan"`

	// Interface is the PON port (e.g., "0/0/1")
	Interface string `json:"interface"`

	// ONTID is the ONT ID on the PON port
	ONTID int `json:"ont_id"`

	// GemPort is the GEM port ID
	GemPort int `json:"gemport"`

	// UserVLAN is the user-side VLAN (C-VLAN)
	UserVLAN int `json:"user_vlan"`

	// TagTransform is the VLAN tag transformation mode
	// Values: "translate", "transparent", "tag"
	TagTransform string `json:"tag_transform"`

	// ETHPort is the ONT Ethernet port (default: 1)
	ETHPort int `json:"eth_port,omitempty"`

	// Metadata contains vendor-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CreateVLANRequest contains parameters for creating a VLAN.
type CreateVLANRequest struct {
	// ID is the VLAN ID (1-4094)
	ID int `json:"id"`

	// Name is the VLAN name (optional)
	Name string `json:"name,omitempty"`

	// Description is the VLAN description (optional)
	Description string `json:"description,omitempty"`

	// Type is the VLAN type (default: "smart")
	Type string `json:"type,omitempty"`
}

// AddServicePortRequest contains parameters for adding a service port.
type AddServicePortRequest struct {
	// VLAN is the service VLAN ID
	VLAN int `json:"vlan"`

	// PONPort is the PON port (e.g., "0/0/1")
	PONPort string `json:"pon_port"`

	// ONTID is the ONT ID on the PON port
	ONTID int `json:"ont_id"`

	// GemPort is the GEM port ID (default: 1)
	GemPort int `json:"gemport,omitempty"`

	// UserVLAN is the user-side VLAN (default: same as VLAN)
	UserVLAN int `json:"user_vlan,omitempty"`

	// TagTransform is the VLAN tag transformation mode (default: "translate")
	TagTransform string `json:"tag_transform,omitempty"`

	// ETHPort is the ONT Ethernet port (default: 1)
	ETHPort int `json:"eth_port,omitempty"`
}

// VLAN error codes
const (
	ErrCodeVLANExists          = "VLAN_EXISTS"
	ErrCodeVLANNotFound        = "VLAN_NOT_FOUND"
	ErrCodeInvalidVLANID       = "INVALID_VLAN_ID"
	ErrCodeVLANHasServicePorts = "VLAN_HAS_SERVICE_PORTS"
	ErrCodeServicePortExists   = "SERVICE_PORT_EXISTS"
)
