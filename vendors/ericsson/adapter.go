package ericsson

import (
	"context"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
)

// Adapter wraps a base driver with Ericsson-specific logic
// Ericsson Router 6000 series uses NETCONF/YANG
type Adapter struct {
	baseDriver types.Driver
	config     *types.EquipmentConfig
}

func NewAdapter(baseDriver types.Driver, config *types.EquipmentConfig) types.Driver {
	return &Adapter{baseDriver: baseDriver, config: config}
}

func (a *Adapter) Connect(ctx context.Context, config *types.EquipmentConfig) error {
	return a.baseDriver.Connect(ctx, config)
}

func (a *Adapter) Disconnect(ctx context.Context) error {
	return a.baseDriver.Disconnect(ctx)
}

func (a *Adapter) IsConnected() bool {
	return a.baseDriver.IsConnected()
}

func (a *Adapter) CreateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) (*types.SubscriberResult, error) {
	result, err := a.baseDriver.CreateSubscriber(ctx, subscriber, tier)
	if err == nil && result != nil {
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}
		result.Metadata["vendor"] = "ericsson"
	}
	return result, err
}

func (a *Adapter) UpdateSubscriber(ctx context.Context, subscriber *model.Subscriber, tier *model.ServiceTier) error {
	return a.baseDriver.UpdateSubscriber(ctx, subscriber, tier)
}

func (a *Adapter) DeleteSubscriber(ctx context.Context, subscriberID string) error {
	return a.baseDriver.DeleteSubscriber(ctx, subscriberID)
}

func (a *Adapter) SuspendSubscriber(ctx context.Context, subscriberID string) error {
	return a.baseDriver.SuspendSubscriber(ctx, subscriberID)
}

func (a *Adapter) ResumeSubscriber(ctx context.Context, subscriberID string) error {
	return a.baseDriver.ResumeSubscriber(ctx, subscriberID)
}

func (a *Adapter) GetSubscriberStatus(ctx context.Context, subscriberID string) (*types.SubscriberStatus, error) {
	return a.baseDriver.GetSubscriberStatus(ctx, subscriberID)
}

func (a *Adapter) GetSubscriberStats(ctx context.Context, subscriberID string) (*types.SubscriberStats, error) {
	return a.baseDriver.GetSubscriberStats(ctx, subscriberID)
}

func (a *Adapter) HealthCheck(ctx context.Context) error {
	return a.baseDriver.HealthCheck(ctx)
}
