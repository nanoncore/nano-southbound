package vsol

import (
	"context"
	"fmt"
	"testing"
)

type fakeSNMPExecutor struct {
	walks map[string]map[string]interface{}
}

func (f *fakeSNMPExecutor) GetSNMP(_ context.Context, _ string) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeSNMPExecutor) WalkSNMP(_ context.Context, oid string) (map[string]interface{}, error) {
	if values, ok := f.walks[oid]; ok {
		return values, nil
	}
	return map[string]interface{}{}, nil
}

func (f *fakeSNMPExecutor) BulkGetSNMP(_ context.Context, _ []string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func TestGetONUListSNMPLineProfile(t *testing.T) {
	executor := &fakeSNMPExecutor{
		walks: map[string]map[string]interface{}{
			OIDONUSerialNumber: {
				".1.6": "FHTT59CB8310",
			},
			OIDONULineProfile: {
				".1.6": "line_vlan_999",
			},
			OIDONUProfile: {
				".1.6": "AN5506-04-F1",
			},
		},
	}

	adapter := &Adapter{
		snmpExecutor: executor,
	}

	results, err := adapter.getONUListSNMP(context.Background())
	if err != nil {
		t.Fatalf("getONUListSNMP returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 ONU, got %d", len(results))
	}

	onu := results[0]
	if onu.PONPort != "0/1" {
		t.Fatalf("expected PON port 0/1, got %q", onu.PONPort)
	}
	if onu.ONUID != 6 {
		t.Fatalf("expected ONU ID 6, got %d", onu.ONUID)
	}
	if onu.LineProfile != "line_vlan_999" {
		t.Fatalf("expected line profile line_vlan_999, got %q", onu.LineProfile)
	}
	if onu.Metadata == nil {
		t.Fatalf("expected metadata to be populated")
	}
	if profile, ok := onu.Metadata["onu_profile"]; !ok || profile != "AN5506-04-F1" {
		t.Fatalf("expected ONU profile metadata AN5506-04-F1, got %v", onu.Metadata["onu_profile"])
	}
}
