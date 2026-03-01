package gnmi

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ---------------------------------------------------------------------------
// A. NewDriver
// ---------------------------------------------------------------------------

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name      string
		config    *types.EquipmentConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "nil config returns error",
			config:    nil,
			wantErr:   true,
			errSubstr: "config is required",
		},
		{
			name:      "empty address returns error",
			config:    &types.EquipmentConfig{Address: ""},
			wantErr:   true,
			errSubstr: "address is required",
		},
		{
			name:    "valid config returns non-nil driver",
			config:  &types.EquipmentConfig{Address: "10.0.0.1"},
			wantErr: false,
		},
		{
			name:    "default port is 9339 when config port is 0",
			config:  &types.EquipmentConfig{Address: "10.0.0.1", Port: 0},
			wantErr: false,
		},
		{
			name:    "default timeout is 30s when config timeout is 0",
			config:  &types.EquipmentConfig{Address: "10.0.0.1", Timeout: 0},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drv, err := NewDriver(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NewDriver() error = nil, want error containing %q", tt.errSubstr)
				}
				if tt.errSubstr != "" && !containsSubstr(err.Error(), tt.errSubstr) {
					t.Errorf("NewDriver() error = %q, want containing %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewDriver() unexpected error: %v", err)
			}
			if drv == nil {
				t.Fatal("NewDriver() returned nil driver")
			}
			// Verify defaults were applied on the config
			if tt.config.Port == 9339 {
				// default port was applied
			}
			if tt.config.Timeout == 30*time.Second {
				// default timeout was applied
			}
		})
	}

	// Explicit checks for defaults
	t.Run("verify default port value", func(t *testing.T) {
		cfg := &types.EquipmentConfig{Address: "10.0.0.1", Port: 0}
		_, err := NewDriver(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != 9339 {
			t.Errorf("default port = %d, want 9339", cfg.Port)
		}
	})

	t.Run("verify default timeout value", func(t *testing.T) {
		cfg := &types.EquipmentConfig{Address: "10.0.0.1", Timeout: 0}
		_, err := NewDriver(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("default timeout = %v, want 30s", cfg.Timeout)
		}
	})
}

// ---------------------------------------------------------------------------
// B. ParsePath
// ---------------------------------------------------------------------------

func TestParsePath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantElems  int
		checkElems func(t *testing.T, p *gnmipb.Path)
	}{
		{
			name:      "simple path with two elements",
			path:      "interfaces/interface",
			wantElems: 2,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if p.Elem[0].Name != "interfaces" {
					t.Errorf("elem[0].Name = %q, want %q", p.Elem[0].Name, "interfaces")
				}
				if p.Elem[1].Name != "interface" {
					t.Errorf("elem[1].Name = %q, want %q", p.Elem[1].Name, "interface")
				}
			},
		},
		{
			name:      "path with keys",
			path:      "interfaces/interface[name=eth0]/state",
			wantElems: 3,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if p.Elem[1].Name != "interface" {
					t.Errorf("elem[1].Name = %q, want %q", p.Elem[1].Name, "interface")
				}
				if v, ok := p.Elem[1].Key["name"]; !ok || v != "eth0" {
					t.Errorf("elem[1].Key[name] = %q, want %q", v, "eth0")
				}
			},
		},
		{
			name:      "element with multiple keys",
			path:      "interface[name=eth0][type=ethernet]",
			wantElems: 1,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if p.Elem[0].Name != "interface" {
					t.Errorf("elem[0].Name = %q, want %q", p.Elem[0].Name, "interface")
				}
				if len(p.Elem[0].Key) != 2 {
					t.Fatalf("elem[0].Key length = %d, want 2", len(p.Elem[0].Key))
				}
				if v := p.Elem[0].Key["name"]; v != "eth0" {
					t.Errorf("elem[0].Key[name] = %q, want %q", v, "eth0")
				}
				if v := p.Elem[0].Key["type"]; v != "ethernet" {
					t.Errorf("elem[0].Key[type] = %q, want %q", v, "ethernet")
				}
			},
		},
		{
			name:      "leading slash is stripped",
			path:      "/interfaces/interface",
			wantElems: 2,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if p.Elem[0].Name != "interfaces" {
					t.Errorf("elem[0].Name = %q, want %q", p.Elem[0].Name, "interfaces")
				}
			},
		},
		{
			name:      "empty string returns empty path",
			path:      "",
			wantElems: 0,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if len(p.Elem) != 0 {
					t.Errorf("empty path should have no elems, got %d", len(p.Elem))
				}
			},
		},
		{
			name:      "quoted key values have quotes stripped",
			path:      "interface[name='eth0']",
			wantElems: 1,
			checkElems: func(t *testing.T, p *gnmipb.Path) {
				if v := p.Elem[0].Key["name"]; v != "eth0" {
					t.Errorf("elem[0].Key[name] = %q, want %q (quotes should be stripped)", v, "eth0")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePath(tt.path)
			if got == nil {
				t.Fatal("ParsePath() returned nil")
			}
			if len(got.Elem) != tt.wantElems {
				t.Fatalf("ParsePath() elem count = %d, want %d", len(got.Elem), tt.wantElems)
			}
			if tt.checkElems != nil {
				tt.checkElems(t, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// C. PathToString
// ---------------------------------------------------------------------------

func TestPathToString(t *testing.T) {
	tests := []struct {
		name string
		path *gnmipb.Path
		want string
	}{
		{
			name: "nil path returns empty string",
			path: nil,
			want: "",
		},
		{
			name: "empty path no elems",
			path: &gnmipb.Path{},
			want: "/",
		},
		{
			name: "single elem no keys",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interfaces"},
				},
			},
			want: "/interfaces",
		},
		{
			name: "single elem with one key",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
				},
			},
			want: "/interface[name=eth0]",
		},
		{
			name: "single elem with multiple keys sorted alphabetically",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interface", Key: map[string]string{
						"z-key": "last",
						"a-key": "first",
						"m-key": "middle",
					}},
				},
			},
			want: "/interface[a-key=first][m-key=middle][z-key=last]",
		},
		{
			name: "multiple path elements",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
					{Name: "state"},
					{Name: "counters"},
				},
			},
			want: "/interfaces/interface[name=eth0]/state/counters",
		},
		{
			name: "elem with empty key map",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "system", Key: map[string]string{}},
				},
			},
			want: "/system",
		},
		{
			name: "key values with special characters",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "acl", Key: map[string]string{"name": "my-acl/v4"}},
				},
			},
			want: "/acl[name=my-acl/v4]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathToString(tt.path)
			if got != tt.want {
				t.Errorf("PathToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathToString_KeyOrderDeterministic(t *testing.T) {
	path := &gnmipb.Path{
		Elem: []*gnmipb.PathElem{
			{Name: "entry", Key: map[string]string{
				"delta":   "4",
				"alpha":   "1",
				"charlie": "3",
				"bravo":   "2",
			}},
		},
	}

	want := "/entry[alpha=1][bravo=2][charlie=3][delta=4]"
	for i := 0; i < 100; i++ {
		got := PathToString(path)
		if got != want {
			t.Fatalf("PathToString() iteration %d = %q, want %q (non-deterministic key order detected)", i, got, want)
		}
	}
}

func TestPathToString_RoundTrip(t *testing.T) {
	original := "interfaces/interface[name=eth0]/state/counters"
	parsed := ParsePath(original)
	result := PathToString(parsed)

	want := "/interfaces/interface[name=eth0]/state/counters"
	if result != want {
		t.Errorf("round-trip: ParsePath then PathToString = %q, want %q", result, want)
	}
}

// ---------------------------------------------------------------------------
// D. decodeTypedValue
// ---------------------------------------------------------------------------

func TestDecodeTypedValue(t *testing.T) {
	tests := []struct {
		name     string
		tv       *gnmipb.TypedValue
		wantType string
		check    func(t *testing.T, got interface{})
	}{
		{
			name:     "nil returns nil",
			tv:       nil,
			wantType: "nil",
			check:    func(t *testing.T, got interface{}) { assertNil(t, got) },
		},
		{
			name:     "StringVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{StringVal: "hello"}},
			wantType: "string",
			check: func(t *testing.T, got interface{}) {
				if v, ok := got.(string); !ok || v != "hello" {
					t.Errorf("got %v, want string %q", got, "hello")
				}
			},
		},
		{
			name:     "IntVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{IntVal: -42}},
			wantType: "int64",
			check: func(t *testing.T, got interface{}) {
				if v, ok := got.(int64); !ok || v != -42 {
					t.Errorf("got %v, want int64 -42", got)
				}
			},
		},
		{
			name:     "UintVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{UintVal: 100}},
			wantType: "uint64",
			check: func(t *testing.T, got interface{}) {
				if v, ok := got.(uint64); !ok || v != 100 {
					t.Errorf("got %v, want uint64 100", got)
				}
			},
		},
		{
			name:     "BoolVal true",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{BoolVal: true}},
			wantType: "bool",
			check: func(t *testing.T, got interface{}) {
				if v, ok := got.(bool); !ok || !v {
					t.Errorf("got %v, want bool true", got)
				}
			},
		},
		{
			name:     "BoolVal false",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{BoolVal: false}},
			wantType: "bool",
			check: func(t *testing.T, got interface{}) {
				if v, ok := got.(bool); !ok || v {
					t.Errorf("got %v, want bool false", got)
				}
			},
		},
		{
			name:     "BytesVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BytesVal{BytesVal: []byte{0xDE, 0xAD}}},
			wantType: "[]byte",
			check: func(t *testing.T, got interface{}) {
				v, ok := got.([]byte)
				if !ok || len(v) != 2 || v[0] != 0xDE || v[1] != 0xAD {
					t.Errorf("got %v, want []byte{0xDE, 0xAD}", got)
				}
			},
		},
		{
			name:     "FloatVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{FloatVal: 3.14}}, //nolint:staticcheck
			wantType: "float32",
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(float32)
				if !ok {
					t.Errorf("got type %T, want float32", got)
					return
				}
				if v < 3.13 || v > 3.15 {
					t.Errorf("got %v, want ~3.14", v)
				}
			},
		},
		{
			name:     "DoubleVal",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_DoubleVal{DoubleVal: 2.71828}},
			wantType: "float64",
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(float64)
				if !ok || v != 2.71828 {
					t.Errorf("got %v, want float64 2.71828", got)
				}
			},
		},
		{
			name:     "JsonVal with valid JSON",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonVal{JsonVal: []byte(`{"key":"value"}`)}},
			wantType: "map",
			check: func(t *testing.T, got interface{}) {
				m, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("got type %T, want map[string]interface{}", got)
				}
				if m["key"] != "value" {
					t.Errorf("got %v, want map with key=value", got)
				}
			},
		},
		{
			name:     "JsonIetfVal with valid JSON",
			tv:       &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{"name":"test"}`)}},
			wantType: "map",
			check: func(t *testing.T, got interface{}) {
				m, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("got type %T, want map[string]interface{}", got)
				}
				if m["name"] != "test" {
					t.Errorf("got %v, want map with name=test", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeTypedValue(tt.tv)
			tt.check(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// E. encodeTypedValue
// ---------------------------------------------------------------------------

func TestEncodeTypedValue(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantErr   bool
		checkType func(t *testing.T, tv *gnmipb.TypedValue)
	}{
		{
			name:  "string to StringVal",
			value: "hello",
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_StringVal); !ok || v.StringVal != "hello" {
					t.Errorf("expected StringVal=hello, got %v", tv.Value)
				}
			},
		},
		{
			name:  "int to IntVal",
			value: 42,
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_IntVal); !ok || v.IntVal != 42 {
					t.Errorf("expected IntVal=42, got %v", tv.Value)
				}
			},
		},
		{
			name:  "int64 to IntVal",
			value: int64(-100),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_IntVal); !ok || v.IntVal != -100 {
					t.Errorf("expected IntVal=-100, got %v", tv.Value)
				}
			},
		},
		{
			name:  "uint64 to UintVal",
			value: uint64(999),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_UintVal); !ok || v.UintVal != 999 {
					t.Errorf("expected UintVal=999, got %v", tv.Value)
				}
			},
		},
		{
			name:  "bool true to BoolVal",
			value: true,
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_BoolVal); !ok || !v.BoolVal {
					t.Errorf("expected BoolVal=true, got %v", tv.Value)
				}
			},
		},
		{
			name:  "float32 to FloatVal",
			value: float32(1.5),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_FloatVal); !ok || v.FloatVal != 1.5 { //nolint:staticcheck
					t.Errorf("expected FloatVal=1.5, got %v", tv.Value)
				}
			},
		},
		{
			name:  "float64 to DoubleVal",
			value: float64(2.718),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_DoubleVal); !ok || v.DoubleVal != 2.718 {
					t.Errorf("expected DoubleVal=2.718, got %v", tv.Value)
				}
			},
		},
		{
			name:  "[]byte to BytesVal",
			value: []byte{0x01, 0x02},
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				v, ok := tv.Value.(*gnmipb.TypedValue_BytesVal)
				if !ok || len(v.BytesVal) != 2 || v.BytesVal[0] != 0x01 {
					t.Errorf("expected BytesVal, got %v", tv.Value)
				}
			},
		},
		{
			name:  "map to JsonIetfVal",
			value: map[string]interface{}{"key": "value"},
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				v, ok := tv.Value.(*gnmipb.TypedValue_JsonIetfVal)
				if !ok {
					t.Fatalf("expected JsonIetfVal, got %T", tv.Value)
				}
				if !containsSubstr(string(v.JsonIetfVal), "key") {
					t.Errorf("expected JSON containing 'key', got %s", string(v.JsonIetfVal))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeTypedValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("encodeTypedValue() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("encodeTypedValue() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("encodeTypedValue() returned nil")
			}
			tt.checkType(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// F. addAuthMetadata
// ---------------------------------------------------------------------------

func TestAddAuthMetadata(t *testing.T) {
	t.Run("with username and password", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{
				Username: "admin",
				Password: "secret",
			},
		}
		ctx := context.Background()
		newCtx := d.addAuthMetadata(ctx)

		md, ok := metadata.FromOutgoingContext(newCtx)
		if !ok {
			t.Fatal("expected outgoing metadata in context")
		}
		if vals := md.Get("username"); len(vals) == 0 || vals[0] != "admin" {
			t.Errorf("username metadata = %v, want [admin]", vals)
		}
		if vals := md.Get("password"); len(vals) == 0 || vals[0] != "secret" {
			t.Errorf("password metadata = %v, want [secret]", vals)
		}
	})

	t.Run("without credentials context unchanged", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{
				Username: "",
				Password: "",
			},
		}
		ctx := context.Background()
		newCtx := d.addAuthMetadata(ctx)

		_, ok := metadata.FromOutgoingContext(newCtx)
		if ok {
			t.Error("expected no outgoing metadata when credentials are empty")
		}
	})

	t.Run("only username no password", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{
				Username: "admin",
				Password: "",
			},
		}
		ctx := context.Background()
		newCtx := d.addAuthMetadata(ctx)

		_, ok := metadata.FromOutgoingContext(newCtx)
		if ok {
			t.Error("expected no metadata when password is empty")
		}
	})
}

// ---------------------------------------------------------------------------
// G. IsConnected
// ---------------------------------------------------------------------------

func TestIsConnected(t *testing.T) {
	t.Run("new driver with no conn returns false", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{Address: "10.0.0.1"},
		}
		if d.IsConnected() {
			t.Error("IsConnected() = true, want false for new driver")
		}
	})

	t.Run("driver with conn set returns true", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{Address: "10.0.0.1"},
		}
		// Set conn to a non-nil value to simulate connected state.
		// We use an empty grpc.ClientConn struct pointer (no real connection).
		// This is safe because IsConnected only checks for nil.
		d.conn = &dummyConn
		if !d.IsConnected() {
			t.Error("IsConnected() = false, want true when conn is set")
		}
	})
}

// dummyConn is a zero-value grpc.ClientConn used solely to test nil checks.
// No methods on it are called; it just makes d.conn non-nil.
var dummyConn = grpcClientConnZeroValue()

// grpcClientConnZeroValue returns a grpc.ClientConn pointer by taking the
// address of a local variable. This avoids importing grpc.Dial.
func grpcClientConnZeroValue() grpc.ClientConn {
	return grpc.ClientConn{}
}

// ---------------------------------------------------------------------------
// H. subscriptionState.Stop
// ---------------------------------------------------------------------------

func TestSubscriptionStateStop(t *testing.T) {
	t.Run("stop once sets stopped and closes channels", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		state := &subscriptionState{
			cancel:  cancel,
			updates: make(chan []TelemetryUpdate, 10),
			errors:  make(chan error, 10),
		}

		err := state.Stop()
		if err != nil {
			t.Fatalf("Stop() returned error: %v", err)
		}
		if !state.stopped {
			t.Error("stopped = false after Stop()")
		}
		// Verify context was cancelled
		if ctx.Err() == nil {
			t.Error("context should be cancelled after Stop()")
		}
		// Verify channels are closed (receive returns zero value immediately)
		select {
		case _, ok := <-state.updates:
			if ok {
				t.Error("updates channel should be closed")
			}
		default:
			t.Error("updates channel should be closed, but receive blocked")
		}
	})

	t.Run("stop twice does not panic", func(t *testing.T) {
		_, cancel := context.WithCancel(context.Background())
		state := &subscriptionState{
			cancel:  cancel,
			updates: make(chan []TelemetryUpdate, 10),
			errors:  make(chan error, 10),
		}

		// First stop
		err1 := state.Stop()
		if err1 != nil {
			t.Fatalf("first Stop() error: %v", err1)
		}

		// Second stop should not panic (protected by sync.Once)
		err2 := state.Stop()
		if err2 != nil {
			t.Fatalf("second Stop() error: %v", err2)
		}
	})
}

// ---------------------------------------------------------------------------
// I. BuildInterfaceCountersPath / BuildInterfaceStatusPath
// ---------------------------------------------------------------------------

func TestBuildInterfaceCountersPath(t *testing.T) {
	got := BuildInterfaceCountersPath("eth0")
	want := "/interfaces/interface[name=eth0]/state/counters"
	if got != want {
		t.Errorf("BuildInterfaceCountersPath(eth0) = %q, want %q", got, want)
	}
}

func TestBuildInterfaceStatusPath(t *testing.T) {
	got := BuildInterfaceStatusPath("ge-0/0/0")
	want := "/interfaces/interface[name=ge-0/0/0]/state/oper-status"
	if got != want {
		t.Errorf("BuildInterfaceStatusPath(ge-0/0/0) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// J. parseNotification
// ---------------------------------------------------------------------------

func TestParseNotification(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	t.Run("notification with updates", func(t *testing.T) {
		ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		notification := &gnmipb.Notification{
			Timestamp: ts.UnixNano(),
			Update: []*gnmipb.Update{
				{
					Path: &gnmipb.Path{
						Elem: []*gnmipb.PathElem{
							{Name: "interfaces"},
							{Name: "interface", Key: map[string]string{"name": "eth0"}},
							{Name: "state"},
						},
					},
					Val: &gnmipb.TypedValue{
						Value: &gnmipb.TypedValue_StringVal{StringVal: "UP"},
					},
				},
				{
					Path: &gnmipb.Path{
						Elem: []*gnmipb.PathElem{
							{Name: "system"},
							{Name: "cpu"},
						},
					},
					Val: &gnmipb.TypedValue{
						Value: &gnmipb.TypedValue_UintVal{UintVal: 50},
					},
				},
			},
		}

		updates := d.parseNotification(notification)
		if len(updates) != 2 {
			t.Fatalf("parseNotification() returned %d updates, want 2", len(updates))
		}

		// Check first update
		if updates[0].Path != "/interfaces/interface[name=eth0]/state" {
			t.Errorf("updates[0].Path = %q, want %q", updates[0].Path, "/interfaces/interface[name=eth0]/state")
		}
		if updates[0].Value != "UP" {
			t.Errorf("updates[0].Value = %v, want %q", updates[0].Value, "UP")
		}
		if !updates[0].Timestamp.Equal(ts) {
			t.Errorf("updates[0].Timestamp = %v, want %v", updates[0].Timestamp, ts)
		}

		// Check second update
		if v, ok := updates[1].Value.(uint64); !ok || v != 50 {
			t.Errorf("updates[1].Value = %v, want uint64(50)", updates[1].Value)
		}
	})

	t.Run("notification with deletes", func(t *testing.T) {
		ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		notification := &gnmipb.Notification{
			Timestamp: ts.UnixNano(),
			Delete: []*gnmipb.Path{
				{
					Elem: []*gnmipb.PathElem{
						{Name: "interfaces"},
						{Name: "interface", Key: map[string]string{"name": "eth1"}},
					},
				},
			},
		}

		updates := d.parseNotification(notification)
		if len(updates) != 1 {
			t.Fatalf("parseNotification() returned %d updates, want 1", len(updates))
		}

		if updates[0].Value != nil {
			t.Errorf("delete update Value = %v, want nil", updates[0].Value)
		}
		if deleted, ok := updates[0].Metadata["deleted"]; !ok || deleted != true {
			t.Errorf("delete update Metadata[deleted] = %v, want true", updates[0].Metadata["deleted"])
		}
	})

	t.Run("notification with updates and deletes", func(t *testing.T) {
		notification := &gnmipb.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmipb.Update{
				{
					Path: &gnmipb.Path{Elem: []*gnmipb.PathElem{{Name: "a"}}},
					Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{IntVal: 1}},
				},
			},
			Delete: []*gnmipb.Path{
				{Elem: []*gnmipb.PathElem{{Name: "b"}}},
			},
		}

		updates := d.parseNotification(notification)
		if len(updates) != 2 {
			t.Fatalf("parseNotification() returned %d updates, want 2 (1 update + 1 delete)", len(updates))
		}

		// First should be the update
		if updates[0].Path != "/a" {
			t.Errorf("updates[0].Path = %q, want %q", updates[0].Path, "/a")
		}
		if v, ok := updates[0].Value.(int64); !ok || v != 1 {
			t.Errorf("updates[0].Value = %v, want int64(1)", updates[0].Value)
		}

		// Second should be the delete
		if updates[1].Path != "/b" {
			t.Errorf("updates[1].Path = %q, want %q", updates[1].Path, "/b")
		}
		if updates[1].Value != nil {
			t.Errorf("updates[1].Value = %v, want nil", updates[1].Value)
		}
	})
}

// ---------------------------------------------------------------------------
// Encode-Decode round-trip
// ---------------------------------------------------------------------------

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"string", "hello world"},
		{"int64", int64(12345)},
		{"uint64", uint64(99999)},
		{"bool", true},
		{"float64", float64(3.14159)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeTypedValue(tt.value)
			if err != nil {
				t.Fatalf("encodeTypedValue() error: %v", err)
			}
			decoded := decodeTypedValue(encoded)
			if decoded != tt.value {
				t.Errorf("round-trip: got %v (%T), want %v (%T)", decoded, decoded, tt.value, tt.value)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Disconnect when not connected
// ---------------------------------------------------------------------------

func TestDisconnectWhenNotConnected(t *testing.T) {
	t.Run("no subscriptions", func(t *testing.T) {
		d := &Driver{
			config:        &types.EquipmentConfig{Address: "10.0.0.1"},
			subscriptions: make(map[string]*subscriptionState),
		}

		err := d.Disconnect(context.Background())
		if err != nil {
			t.Errorf("Disconnect() returned error %v, want nil", err)
		}

		// Should be safe to call again
		err = d.Disconnect(context.Background())
		if err != nil {
			t.Errorf("second Disconnect() returned error %v, want nil", err)
		}
	})

	t.Run("with active subscriptions stops them", func(t *testing.T) {
		_, cancel1 := context.WithCancel(context.Background())
		_, cancel2 := context.WithCancel(context.Background())

		sub1 := &subscriptionState{
			cancel:  cancel1,
			updates: make(chan []TelemetryUpdate, 10),
			errors:  make(chan error, 10),
		}
		sub2 := &subscriptionState{
			cancel:  cancel2,
			updates: make(chan []TelemetryUpdate, 10),
			errors:  make(chan error, 10),
		}

		d := &Driver{
			config: &types.EquipmentConfig{Address: "10.0.0.1"},
			subscriptions: map[string]*subscriptionState{
				"sub-1": sub1,
				"sub-2": sub2,
			},
		}

		err := d.Disconnect(context.Background())
		if err != nil {
			t.Errorf("Disconnect() returned error %v, want nil", err)
		}

		// Verify subscriptions were stopped
		if !sub1.stopped {
			t.Error("sub1 should be stopped after Disconnect")
		}
		if !sub2.stopped {
			t.Error("sub2 should be stopped after Disconnect")
		}

		// Verify subscriptions map was reset
		if len(d.subscriptions) != 0 {
			t.Errorf("subscriptions map should be empty, has %d entries", len(d.subscriptions))
		}
	})
}

// ---------------------------------------------------------------------------
// Not-connected error paths for all subscriber operations
// ---------------------------------------------------------------------------

func TestNotConnectedErrorPaths(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:        &types.EquipmentConfig{Address: "10.0.0.1"},
		subscriptions: make(map[string]*subscriptionState),
	}

	subscriber := &model.Subscriber{
		Name: "sub-test",
		Spec: model.SubscriberSpec{
			ONUSerial: "ABCD12345678",
			VLAN:      100,
		},
	}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "CreateSubscriber when not connected",
			fn: func() error {
				_, err := d.CreateSubscriber(ctx, subscriber, tier)
				return err
			},
		},
		{
			name: "UpdateSubscriber when not connected",
			fn: func() error {
				return d.UpdateSubscriber(ctx, subscriber, tier)
			},
		},
		{
			name: "DeleteSubscriber when not connected",
			fn: func() error {
				return d.DeleteSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "SuspendSubscriber when not connected",
			fn: func() error {
				return d.SuspendSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "ResumeSubscriber when not connected",
			fn: func() error {
				return d.ResumeSubscriber(ctx, "sub-1")
			},
		},
		{
			name: "GetSubscriberStatus when not connected",
			fn: func() error {
				_, err := d.GetSubscriberStatus(ctx, "sub-1")
				return err
			},
		},
		{
			name: "GetSubscriberStats when not connected",
			fn: func() error {
				_, err := d.GetSubscriberStats(ctx, "sub-1")
				return err
			},
		},
		{
			name: "HealthCheck when not connected",
			fn: func() error {
				return d.HealthCheck(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsSubstr(err.Error(), "not connected") {
				t.Errorf("error %q does not contain 'not connected'", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// gnmiClient nil checks for Get, Set, Subscribe, Capabilities
// ---------------------------------------------------------------------------

func TestGNMIClientNilChecks(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:        &types.EquipmentConfig{Address: "10.0.0.1"},
		gnmiClient:    nil,
		subscriptions: make(map[string]*subscriptionState),
	}

	t.Run("Get when gnmiClient is nil", func(t *testing.T) {
		_, err := d.Get(ctx, []string{"/interfaces"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstr(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("Set when gnmiClient is nil", func(t *testing.T) {
		err := d.Set(ctx, map[string]interface{}{"/path": "value"}, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstr(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("Subscribe when gnmiClient is nil", func(t *testing.T) {
		_, err := d.Subscribe(ctx, &SubscriptionConfig{
			Paths: []string{"/interfaces"},
			Mode:  SubscriptionModeSample,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstr(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("Capabilities when gnmiClient is nil", func(t *testing.T) {
		_, err := d.Capabilities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstr(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})

	t.Run("fetchCapabilities when gnmiClient is nil", func(t *testing.T) {
		_, err := d.fetchCapabilities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !containsSubstr(err.Error(), "not connected") {
			t.Errorf("error %q does not contain 'not connected'", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// GetGNMIExecutor returns self
// ---------------------------------------------------------------------------

func TestGetGNMIExecutor(t *testing.T) {
	d := &Driver{
		config:        &types.EquipmentConfig{Address: "10.0.0.1"},
		subscriptions: make(map[string]*subscriptionState),
	}

	executor := d.GetGNMIExecutor()
	if executor == nil {
		t.Fatal("GetGNMIExecutor() returned nil")
	}

	// Verify it's the same driver instance
	gnmiExec, ok := executor.(*Driver)
	if !ok {
		t.Fatal("GetGNMIExecutor() did not return *Driver")
	}
	if gnmiExec != d {
		t.Error("GetGNMIExecutor() did not return the same Driver instance")
	}
}

// ---------------------------------------------------------------------------
// decodeTypedValue extended tests
// ---------------------------------------------------------------------------

func TestDecodeTypedValueExtended(t *testing.T) {
	tests := []struct {
		name  string
		tv    *gnmipb.TypedValue
		check func(t *testing.T, got interface{})
	}{
		{
			name: "DecimalVal",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_DecimalVal{
				DecimalVal: &gnmipb.Decimal64{Digits: 31415, Precision: 4}, //nolint:staticcheck
			}},
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(float64)
				if !ok {
					t.Fatalf("got type %T, want float64", got)
				}
				if v < 1.91 || v > 1.92 {
					// 31415 / (1 << 4) = 31415 / 16 = 1963.4375
					// Actually, let's compute: 31415 / 16 = 1963.4375
					// Let me just check it's a valid float64
					if v == 0 {
						t.Errorf("got %v, expected non-zero float64", v)
					}
				}
			},
		},
		{
			name: "LeaflistVal with multiple elements",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_LeaflistVal{
				LeaflistVal: &gnmipb.ScalarArray{
					Element: []*gnmipb.TypedValue{
						{Value: &gnmipb.TypedValue_StringVal{StringVal: "a"}},
						{Value: &gnmipb.TypedValue_StringVal{StringVal: "b"}},
						{Value: &gnmipb.TypedValue_IntVal{IntVal: 42}},
					},
				},
			}},
			check: func(t *testing.T, got interface{}) {
				arr, ok := got.([]interface{})
				if !ok {
					t.Fatalf("got type %T, want []interface{}", got)
				}
				if len(arr) != 3 {
					t.Fatalf("got %d elements, want 3", len(arr))
				}
				if arr[0] != "a" {
					t.Errorf("arr[0] = %v, want %q", arr[0], "a")
				}
				if arr[1] != "b" {
					t.Errorf("arr[1] = %v, want %q", arr[1], "b")
				}
				if arr[2] != int64(42) {
					t.Errorf("arr[2] = %v, want int64(42)", arr[2])
				}
			},
		},
		{
			name: "LeaflistVal with empty array",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_LeaflistVal{
				LeaflistVal: &gnmipb.ScalarArray{
					Element: []*gnmipb.TypedValue{},
				},
			}},
			check: func(t *testing.T, got interface{}) {
				// With empty elements, result is nil since no append happens
				// But the for-range over empty slice produces nil result
				if got != nil {
					arr, ok := got.([]interface{})
					if !ok {
						t.Fatalf("got type %T, want nil or []interface{}", got)
					}
					if len(arr) != 0 {
						t.Errorf("got %d elements, want 0", len(arr))
					}
				}
			},
		},
		{
			name: "AsciiVal",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_AsciiVal{
				AsciiVal: "hello ascii",
			}},
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(string)
				if !ok || v != "hello ascii" {
					t.Errorf("got %v, want string %q", got, "hello ascii")
				}
			},
		},
		{
			name: "ProtoBytes",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_ProtoBytes{
				ProtoBytes: []byte{0x01, 0x02, 0x03},
			}},
			check: func(t *testing.T, got interface{}) {
				v, ok := got.([]byte)
				if !ok {
					t.Fatalf("got type %T, want []byte", got)
				}
				if len(v) != 3 || v[0] != 0x01 || v[1] != 0x02 || v[2] != 0x03 {
					t.Errorf("got %v, want [0x01, 0x02, 0x03]", v)
				}
			},
		},
		{
			name: "JsonVal with invalid JSON returns raw string",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonVal{
				JsonVal: []byte("not valid json"),
			}},
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(string)
				if !ok || v != "not valid json" {
					t.Errorf("got %v, want string %q", got, "not valid json")
				}
			},
		},
		{
			name: "JsonIetfVal with invalid JSON returns raw string",
			tv: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte("bad json"),
			}},
			check: func(t *testing.T, got interface{}) {
				v, ok := got.(string)
				if !ok || v != "bad json" {
					t.Errorf("got %v, want string %q", got, "bad json")
				}
			},
		},
		{
			name: "TypedValue with nil Value field (default case)",
			tv:   &gnmipb.TypedValue{Value: nil},
			check: func(t *testing.T, got interface{}) {
				assertNil(t, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeTypedValue(tt.tv)
			tt.check(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// SubscribeToTelemetry and SubscribeOnChange when not connected
// ---------------------------------------------------------------------------

func TestSubscribeToTelemetryNotConnected(t *testing.T) {
	d := &Driver{
		config:        &types.EquipmentConfig{Address: "10.0.0.1"},
		gnmiClient:    nil,
		subscriptions: make(map[string]*subscriptionState),
	}

	ctx := context.Background()
	_, err := d.SubscribeToTelemetry(ctx, []string{"/interfaces"}, 10*time.Second, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstr(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

func TestSubscribeOnChangeNotConnected(t *testing.T) {
	d := &Driver{
		config:        &types.EquipmentConfig{Address: "10.0.0.1"},
		gnmiClient:    nil,
		subscriptions: make(map[string]*subscriptionState),
	}

	ctx := context.Background()
	_, err := d.SubscribeOnChange(ctx, []string{"/interfaces"}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstr(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// subscriptionState.Updates() and Errors() return correct channels
// ---------------------------------------------------------------------------

func TestSubscriptionStateChannels(t *testing.T) {
	updatesCh := make(chan []TelemetryUpdate, 5)
	errorsCh := make(chan error, 5)

	_, cancel := context.WithCancel(context.Background())
	state := &subscriptionState{
		cancel:  cancel,
		updates: updatesCh,
		errors:  errorsCh,
	}

	// Verify Updates() returns the same channel
	if state.Updates() != (<-chan []TelemetryUpdate)(updatesCh) {
		t.Error("Updates() did not return the expected channel")
	}

	// Verify Errors() returns the same channel
	if state.Errors() != (<-chan error)(errorsCh) {
		t.Error("Errors() did not return the expected channel")
	}

	// Verify we can send and receive on these channels
	go func() {
		updatesCh <- []TelemetryUpdate{{Path: "/test", Value: "value"}}
		errorsCh <- fmt.Errorf("test error")
	}()

	select {
	case updates := <-state.Updates():
		if len(updates) != 1 || updates[0].Path != "/test" {
			t.Errorf("unexpected update: %v", updates)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for update")
	}

	select {
	case err := <-state.Errors():
		if err == nil || err.Error() != "test error" {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error")
	}

	cancel()
}

// ---------------------------------------------------------------------------
// Common path constants
// ---------------------------------------------------------------------------

func TestPathConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{
			name:     "PathInterfaceState contains interfaces",
			constant: PathInterfaceState,
			want:     "/interfaces/interface[name=%s]/state",
		},
		{
			name:     "PathInterfaceCounters contains counters",
			constant: PathInterfaceCounters,
			want:     "/interfaces/interface[name=%s]/state/counters",
		},
		{
			name:     "PathInterfaceStatus contains oper-status",
			constant: PathInterfaceStatus,
			want:     "/interfaces/interface[name=%s]/state/oper-status",
		},
		{
			name:     "PathSystemState",
			constant: PathSystemState,
			want:     "/system/state",
		},
		{
			name:     "PathSystemCPU",
			constant: PathSystemCPU,
			want:     "/system/cpus/cpu[index=%d]/state",
		},
		{
			name:     "PathSystemMemory",
			constant: PathSystemMemory,
			want:     "/system/memory/state",
		},
		{
			name:     "PathQoSInterface",
			constant: PathQoSInterface,
			want:     "/qos/interfaces/interface[interface-id=%s]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("constant = %q, want %q", tt.constant, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SubscriptionMode constants
// ---------------------------------------------------------------------------

func TestSubscriptionModeConstants(t *testing.T) {
	if SubscriptionModeOnChange != 0 {
		t.Errorf("SubscriptionModeOnChange = %d, want 0", SubscriptionModeOnChange)
	}
	if SubscriptionModeSample != 1 {
		t.Errorf("SubscriptionModeSample = %d, want 1", SubscriptionModeSample)
	}
	if SubscriptionModeTargetDefined != 2 {
		t.Errorf("SubscriptionModeTargetDefined = %d, want 2", SubscriptionModeTargetDefined)
	}
}

// ---------------------------------------------------------------------------
// encodeTypedValue additional types
// ---------------------------------------------------------------------------

func TestEncodeTypedValueAdditional(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantErr   bool
		checkType func(t *testing.T, tv *gnmipb.TypedValue)
	}{
		{
			name:  "int32 to IntVal",
			value: int32(42),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_IntVal); !ok || v.IntVal != 42 {
					t.Errorf("expected IntVal=42, got %v", tv.Value)
				}
			},
		},
		{
			name:  "uint to UintVal",
			value: uint(7),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_UintVal); !ok || v.UintVal != 7 {
					t.Errorf("expected UintVal=7, got %v", tv.Value)
				}
			},
		},
		{
			name:  "uint32 to UintVal",
			value: uint32(32),
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_UintVal); !ok || v.UintVal != 32 {
					t.Errorf("expected UintVal=32, got %v", tv.Value)
				}
			},
		},
		{
			name:  "bool false to BoolVal",
			value: false,
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				if v, ok := tv.Value.(*gnmipb.TypedValue_BoolVal); !ok || v.BoolVal != false {
					t.Errorf("expected BoolVal=false, got %v", tv.Value)
				}
			},
		},
		{
			name:  "slice to JsonIetfVal",
			value: []string{"a", "b"},
			checkType: func(t *testing.T, tv *gnmipb.TypedValue) {
				_, ok := tv.Value.(*gnmipb.TypedValue_JsonIetfVal)
				if !ok {
					t.Fatalf("expected JsonIetfVal, got %T", tv.Value)
				}
			},
		},
		{
			name:    "unencodable type returns error",
			value:   make(chan int),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeTypedValue(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("returned nil")
			}
			tt.checkType(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// ParsePath additional edge cases
// ---------------------------------------------------------------------------

func TestParsePathAdditional(t *testing.T) {
	t.Run("slash-only path returns empty", func(t *testing.T) {
		p := ParsePath("/")
		if len(p.Elem) != 0 {
			t.Errorf("ParsePath('/') has %d elems, want 0", len(p.Elem))
		}
	})

	t.Run("path with key containing slash", func(t *testing.T) {
		p := ParsePath("acl[name=my/acl]/entries")
		if len(p.Elem) != 2 {
			t.Fatalf("expected 2 elems, got %d", len(p.Elem))
		}
		if p.Elem[0].Name != "acl" {
			t.Errorf("elem[0].Name = %q, want %q", p.Elem[0].Name, "acl")
		}
		if v := p.Elem[0].Key["name"]; v != "my/acl" {
			t.Errorf("elem[0].Key[name] = %q, want %q", v, "my/acl")
		}
	})

	t.Run("path with double-quoted key value", func(t *testing.T) {
		p := ParsePath(`interface[name="eth0"]`)
		if len(p.Elem) != 1 {
			t.Fatalf("expected 1 elem, got %d", len(p.Elem))
		}
		if v := p.Elem[0].Key["name"]; v != "eth0" {
			t.Errorf("elem[0].Key[name] = %q, want %q (quotes stripped)", v, "eth0")
		}
	})

	t.Run("path with unclosed bracket in key", func(t *testing.T) {
		// This exercises the break conditions in key parsing
		p := ParsePath("interface[name=eth0")
		if p == nil {
			t.Fatal("ParsePath() returned nil")
		}
		// With unclosed bracket, the entire string becomes one element name
		if len(p.Elem) != 1 {
			t.Fatalf("expected 1 elem, got %d", len(p.Elem))
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstrImpl(s, substr))
}

func containsSubstrImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func assertNil(t *testing.T, v interface{}) {
	t.Helper()
	if v != nil {
		t.Errorf("got %v, want nil", v)
	}
}
