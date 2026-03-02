package ericsson

import (
	"context"
	"fmt"
	"testing"

	"github.com/nanoncore/nano-southbound/testutil"
	"github.com/nanoncore/nano-southbound/types"
)

var _ types.Driver = (*Adapter)(nil)

func TestNewAdapter(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	cfg := testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1")
	adapter := NewAdapter(mock, cfg)
	if adapter == nil {
		t.Fatal("NewAdapter returned nil")
	}
}

func TestConnect_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	cfg := testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1")
	if err := adapter.Connect(context.Background(), cfg); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if len(mock.Calls) == 0 || mock.Calls[0] != "Connect" {
		t.Fatalf("expected Connect call, got %v", mock.Calls)
	}
}

func TestDisconnect_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if err := adapter.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
}

func TestIsConnected_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if !adapter.IsConnected() {
		t.Fatal("expected IsConnected to return true")
	}
	mock.Connected = false
	if adapter.IsConnected() {
		t.Fatal("expected IsConnected to return false")
	}
}

func TestCreateSubscriber_Success(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	sub := testutil.NewTestSubscriber("ERIC12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	result, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}
	if result.Metadata["vendor"] != "ericsson" {
		t.Fatalf("expected vendor=ericsson, got %v", result.Metadata["vendor"])
	}
}

func TestCreateSubscriber_WithExistingMetadata(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	mock.CreateSubscriberResult = &types.SubscriberResult{
		SubscriberID: "test",
		Metadata:     map[string]interface{}{"existing": "value"},
	}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	sub := testutil.NewTestSubscriber("ERIC12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	result, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err != nil {
		t.Fatalf("CreateSubscriber failed: %v", err)
	}
	if result.Metadata["existing"] != "value" {
		t.Fatal("existing metadata was lost")
	}
	if result.Metadata["vendor"] != "ericsson" {
		t.Fatal("vendor metadata not added")
	}
}

func TestCreateSubscriber_BaseError(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	mock.CreateSubscriberError = fmt.Errorf("base error")
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	sub := testutil.NewTestSubscriber("ERIC12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)

	result, err := adapter.CreateSubscriber(context.Background(), sub, tier)
	if err == nil {
		t.Fatal("expected error from CreateSubscriber")
	}
	if result != nil && result.Metadata != nil {
		if _, ok := result.Metadata["vendor"]; ok {
			t.Fatal("vendor metadata should not be added on error")
		}
	}
}

func TestUpdateSubscriber_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	sub := testutil.NewTestSubscriber("ERIC12345678", "0/1", 100)
	tier := testutil.NewTestServiceTier(50, 100)
	if err := adapter.UpdateSubscriber(context.Background(), sub, tier); err != nil {
		t.Fatalf("UpdateSubscriber failed: %v", err)
	}
}

func TestDeleteSubscriber_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if err := adapter.DeleteSubscriber(context.Background(), "sub-1"); err != nil {
		t.Fatalf("DeleteSubscriber failed: %v", err)
	}
}

func TestSuspendSubscriber_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if err := adapter.SuspendSubscriber(context.Background(), "sub-1"); err != nil {
		t.Fatalf("SuspendSubscriber failed: %v", err)
	}
}

func TestResumeSubscriber_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if err := adapter.ResumeSubscriber(context.Background(), "sub-1"); err != nil {
		t.Fatalf("ResumeSubscriber failed: %v", err)
	}
}

func TestGetSubscriberStatus_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	status, err := adapter.GetSubscriberStatus(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("GetSubscriberStatus failed: %v", err)
	}
	if status.SubscriberID != "sub-1" {
		t.Fatalf("expected sub-1, got %s", status.SubscriberID)
	}
}

func TestGetSubscriberStats_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	stats, err := adapter.GetSubscriberStats(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("GetSubscriberStats failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
}

func TestHealthCheck_Delegates(t *testing.T) {
	mock := &testutil.MockDriver{Connected: true}
	adapter := NewAdapter(mock, testutil.NewTestEquipmentConfig(types.VendorEricsson, "10.0.0.1"))
	if err := adapter.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}
