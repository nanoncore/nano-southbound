package testutil

import (
	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// NewTestSubscriber creates a Subscriber fixture for testing.
func NewTestSubscriber(serial, ponPort string, vlan int) *model.Subscriber {
	return &model.Subscriber{
		Name: "test-" + serial,
		Annotations: map[string]string{
			"nano.io/pon-port": ponPort,
		},
		Spec: model.SubscriberSpec{
			ONUSerial: serial,
			VLAN:      vlan,
			Tier:      "default",
		},
	}
}

// NewTestServiceTier creates a ServiceTier fixture for testing.
func NewTestServiceTier(upMbps, downMbps int) *model.ServiceTier {
	return &model.ServiceTier{
		Name: "test-tier",
		Spec: model.ServiceTierSpec{
			BandwidthUp:   upMbps,
			BandwidthDown: downMbps,
			QoSClass:      "best-effort",
		},
	}
}

// NewTestEquipmentConfig creates an EquipmentConfig fixture for testing.
func NewTestEquipmentConfig(vendor types.Vendor, address string) *types.EquipmentConfig {
	return &types.EquipmentConfig{
		Name:     "test-olt",
		Type:     types.EquipmentTypeOLT,
		Vendor:   vendor,
		Address:  address,
		Port:     22,
		Protocol: types.ProtocolCLI,
		Username: "admin",
		Password: "admin",
		Metadata: map[string]string{},
	}
}

// BoolPtr returns a pointer to a bool value.
func BoolPtr(v bool) *bool { return &v }

// IntPtr returns a pointer to an int value.
func IntPtr(v int) *int { return &v }
