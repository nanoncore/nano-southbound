package netconf

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nanoncore/nano-southbound/model"
	"github.com/nanoncore/nano-southbound/types"
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
			name:    "default port is 830 when config port is 0",
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
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
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
		})
	}

	// Explicit checks for defaults
	t.Run("verify default port value", func(t *testing.T) {
		cfg := &types.EquipmentConfig{Address: "10.0.0.1", Port: 0}
		_, err := NewDriver(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != 830 {
			t.Errorf("default port = %d, want 830", cfg.Port)
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
// B. parseHello
// ---------------------------------------------------------------------------

func TestParseHello(t *testing.T) {
	tests := []struct {
		name          string
		data          string
		wantCaps      int
		wantSessionID string
		checkCaps     func(t *testing.T, caps []string)
	}{
		{
			name: "valid hello with capabilities and session-id",
			data: `<?xml version="1.0" encoding="UTF-8"?>
<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <capabilities>
    <capability>urn:ietf:params:netconf:base:1.0</capability>
    <capability>urn:ietf:params:netconf:base:1.1</capability>
    <capability>urn:ietf:params:netconf:capability:candidate:1.0</capability>
  </capabilities>
  <session-id>42</session-id>
</hello>`,
			wantCaps:      3,
			wantSessionID: "42",
			checkCaps: func(t *testing.T, caps []string) {
				if caps[0] != "urn:ietf:params:netconf:base:1.0" {
					t.Errorf("caps[0] = %q, want base:1.0", caps[0])
				}
				if caps[1] != "urn:ietf:params:netconf:base:1.1" {
					t.Errorf("caps[1] = %q, want base:1.1", caps[1])
				}
			},
		},
		{
			name:          "malformed XML returns empty results",
			data:          "this is not xml at all <><>",
			wantCaps:      0,
			wantSessionID: "",
		},
		{
			name:          "empty data returns empty results",
			data:          "",
			wantCaps:      0,
			wantSessionID: "",
		},
		{
			name: "hello without session-id",
			data: `<?xml version="1.0" encoding="UTF-8"?>
<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <capabilities>
    <capability>urn:ietf:params:netconf:base:1.0</capability>
  </capabilities>
</hello>`,
			wantCaps:      1,
			wantSessionID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps, sessionID := parseHello([]byte(tt.data))
			if len(caps) != tt.wantCaps {
				t.Errorf("parseHello() caps count = %d, want %d", len(caps), tt.wantCaps)
			}
			if sessionID != tt.wantSessionID {
				t.Errorf("parseHello() sessionID = %q, want %q", sessionID, tt.wantSessionID)
			}
			if tt.checkCaps != nil {
				tt.checkCaps(t, caps)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// C. extractRPCError
// ---------------------------------------------------------------------------

func TestExtractRPCError(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantParts []string // substrings that must be present in result
	}{
		{
			name: "valid rpc-reply with single rpc-error",
			data: `<?xml version="1.0" encoding="UTF-8"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>invalid-value</error-tag>
    <error-message>The value is not valid</error-message>
  </rpc-error>
</rpc-reply>`,
			wantParts: []string{"application", "invalid-value", "The value is not valid"},
		},
		{
			name: "valid rpc-reply with multiple rpc-errors returns first",
			data: `<?xml version="1.0" encoding="UTF-8"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  <rpc-error>
    <error-type>protocol</error-type>
    <error-tag>missing-element</error-tag>
    <error-message>first error</error-message>
  </rpc-error>
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>data-exists</error-tag>
    <error-message>second error</error-message>
  </rpc-error>
</rpc-reply>`,
			wantParts: []string{"protocol", "missing-element", "first error"},
		},
		{
			name:      "malformed XML returns raw data as string",
			data:      "this is not valid xml <><>!!",
			wantParts: []string{"this is not valid xml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRPCError([]byte(tt.data))
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("extractRPCError() = %q, want containing %q", got, part)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// D. netconfWriter.Write
// ---------------------------------------------------------------------------

func TestNetconfWriter(t *testing.T) {
	t.Run("NETCONF 1.0 EOM framing appends ]]>]]>", func(t *testing.T) {
		var buf bytes.Buffer
		w := &netconfWriter{writer: &buf, useChunk: false}

		data := []byte("<rpc-reply/>")
		n, err := w.Write(data)
		if err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if n != len(data)+len(NetconfFrameEnd) {
			t.Errorf("Write() returned n=%d, want %d", n, len(data)+len(NetconfFrameEnd))
		}

		got := buf.String()
		if !strings.HasSuffix(got, "]]>]]>") {
			t.Errorf("Write() result = %q, want suffix ]]>]]>", got)
		}
		if !strings.HasPrefix(got, "<rpc-reply/>") {
			t.Errorf("Write() result = %q, want prefix <rpc-reply/>", got)
		}
	})

	t.Run("NETCONF 1.1 chunked framing", func(t *testing.T) {
		var buf bytes.Buffer
		w := &netconfWriter{writer: &buf, useChunk: true}

		data := []byte("hello")
		_, err := w.Write(data)
		if err != nil {
			t.Fatalf("Write() error: %v", err)
		}

		got := buf.String()
		// Expected format: \n#<len>\n<data>\n##\n
		want := "\n#5\nhello\n##\n"
		if got != want {
			t.Errorf("Write() chunked = %q, want %q", got, want)
		}
	})

	t.Run("NETCONF 1.1 chunked with longer data", func(t *testing.T) {
		var buf bytes.Buffer
		w := &netconfWriter{writer: &buf, useChunk: true}

		data := []byte("<rpc-reply>some data</rpc-reply>")
		_, err := w.Write(data)
		if err != nil {
			t.Fatalf("Write() error: %v", err)
		}

		got := buf.String()
		expectedPrefix := "\n#32\n"
		if !strings.HasPrefix(got, expectedPrefix) {
			t.Errorf("Write() result starts with %q, want %q", got[:len(expectedPrefix)], expectedPrefix)
		}
		if !strings.HasSuffix(got, "\n##\n") {
			t.Errorf("Write() result = %q, want suffix \\n##\\n", got)
		}
	})
}

// ---------------------------------------------------------------------------
// E. parseChunkedMessage
// ---------------------------------------------------------------------------

func TestParseChunkedMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "single chunk",
			input: "\n#5\nhello\n##\n",
			want:  "hello",
		},
		{
			name:  "multiple chunks",
			input: "\n#5\nhello\n#6\n world\n##\n",
			want:  "hello world",
		},
		{
			name:  "chunk with XML data",
			input: "\n#11\n<rpc-reply>\n#7\n</data>\n##\n",
			want:  "<rpc-reply></data>",
		},
		{
			name:  "large chunk size",
			input: "\n#26\nabcdefghijklmnopqrstuvwxyz\n##\n",
			want:  "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:  "no chunk markers returns raw data",
			input: "just plain data",
			want:  "just plain data",
		},
		{
			name:    "missing size terminator",
			input:   "\n#5",
			wantErr: "malformed NETCONF chunk: missing size terminator",
		},
		{
			name:    "invalid size - not a number",
			input:   "\n#abc\ndata\n##\n",
			wantErr: `malformed NETCONF chunk: invalid size "abc"`,
		},
		{
			name:    "invalid size - zero",
			input:   "\n#0\n\n##\n",
			wantErr: `malformed NETCONF chunk: invalid size "0"`,
		},
		{
			name:    "invalid size - negative",
			input:   "\n#-1\n\n##\n",
			wantErr: `malformed NETCONF chunk: invalid size "-1"`,
		},
		{
			name:    "size exceeds available data",
			input:   "\n#100\nshort\n##\n",
			wantErr: "malformed NETCONF chunk: size 100 exceeds available data",
		},
		{
			name:  "data before first chunk is ignored",
			input: "preamble\n#3\nabc\n##\n",
			want:  "abc",
		},
		{
			name:  "chunk with size having whitespace",
			input: "\n# 5 \nhello\n##\n",
			want:  "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChunkedMessage([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseChunkedMessage() error = nil, wantErr %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("parseChunkedMessage() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseChunkedMessage() unexpected error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("parseChunkedMessage() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// F. HasCapability
// ---------------------------------------------------------------------------

func TestHasCapability(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:base:1.1",
			"urn:ietf:params:netconf:capability:candidate:1.0",
		},
	}

	t.Run("matching capability returns true", func(t *testing.T) {
		if !d.HasCapability("base:1.0") {
			t.Error("HasCapability(base:1.0) = false, want true")
		}
		if !d.HasCapability("candidate") {
			t.Error("HasCapability(candidate) = false, want true")
		}
	})

	t.Run("non-matching capability returns false", func(t *testing.T) {
		if d.HasCapability("xpath") {
			t.Error("HasCapability(xpath) = true, want false")
		}
		if d.HasCapability("nonexistent") {
			t.Error("HasCapability(nonexistent) = true, want false")
		}
	})
}

// ---------------------------------------------------------------------------
// G. GetCapabilities returns a copy
// ---------------------------------------------------------------------------

func TestGetCapabilities(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:base:1.1",
		},
	}

	caps := d.GetCapabilities()
	if len(caps) != 2 {
		t.Fatalf("GetCapabilities() returned %d caps, want 2", len(caps))
	}

	// Modify returned slice and verify internal state is unchanged
	caps[0] = "modified"
	internal := d.GetCapabilities()
	if internal[0] == "modified" {
		t.Error("GetCapabilities() did not return a copy; modifying the returned slice affected internal state")
	}
	if internal[0] != "urn:ietf:params:netconf:base:1.0" {
		t.Errorf("internal caps[0] = %q, want original value", internal[0])
	}
}

// ---------------------------------------------------------------------------
// H. IsConnected
// ---------------------------------------------------------------------------

func TestIsConnected(t *testing.T) {
	t.Run("default driver is not connected", func(t *testing.T) {
		d := &Driver{
			config: &types.EquipmentConfig{Address: "10.0.0.1"},
		}
		if d.IsConnected() {
			t.Error("IsConnected() = true, want false for new driver")
		}
	})

	t.Run("driver with connected=true returns true", func(t *testing.T) {
		d := &Driver{
			config:    &types.EquipmentConfig{Address: "10.0.0.1"},
			connected: true,
		}
		if !d.IsConnected() {
			t.Error("IsConnected() = false, want true")
		}
	})
}

// ---------------------------------------------------------------------------
// I. EditOption functions
// ---------------------------------------------------------------------------

func TestEditOptions(t *testing.T) {
	t.Run("WithMerge sets defaultOperation to merge", func(t *testing.T) {
		opts := &editOptions{}
		WithMerge()(opts)
		if opts.defaultOperation != "merge" {
			t.Errorf("WithMerge() defaultOperation = %q, want %q", opts.defaultOperation, "merge")
		}
	})

	t.Run("WithReplace sets defaultOperation to replace", func(t *testing.T) {
		opts := &editOptions{}
		WithReplace()(opts)
		if opts.defaultOperation != "replace" {
			t.Errorf("WithReplace() defaultOperation = %q, want %q", opts.defaultOperation, "replace")
		}
	})

	t.Run("WithTestThenSet sets testOption to test-then-set", func(t *testing.T) {
		opts := &editOptions{}
		WithTestThenSet()(opts)
		if opts.testOption != "test-then-set" {
			t.Errorf("WithTestThenSet() testOption = %q, want %q", opts.testOption, "test-then-set")
		}
	})

	t.Run("WithRollbackOnError sets errorOption to rollback-on-error", func(t *testing.T) {
		opts := &editOptions{}
		WithRollbackOnError()(opts)
		if opts.errorOption != "rollback-on-error" {
			t.Errorf("WithRollbackOnError() errorOption = %q, want %q", opts.errorOption, "rollback-on-error")
		}
	})

	t.Run("multiple options compose correctly", func(t *testing.T) {
		opts := &editOptions{}
		WithMerge()(opts)
		WithTestThenSet()(opts)
		WithRollbackOnError()(opts)
		if opts.defaultOperation != "merge" {
			t.Errorf("defaultOperation = %q, want %q", opts.defaultOperation, "merge")
		}
		if opts.testOption != "test-then-set" {
			t.Errorf("testOption = %q, want %q", opts.testOption, "test-then-set")
		}
		if opts.errorOption != "rollback-on-error" {
			t.Errorf("errorOption = %q, want %q", opts.errorOption, "rollback-on-error")
		}
	})
}

// ---------------------------------------------------------------------------
// J. nextMessageID
// ---------------------------------------------------------------------------

func TestNextMessageID(t *testing.T) {
	// Call multiple times, verify monotonically increasing
	first := nextMessageID()
	second := nextMessageID()
	third := nextMessageID()

	if second <= first {
		t.Errorf("nextMessageID() not monotonically increasing: %d then %d", first, second)
	}
	if third <= second {
		t.Errorf("nextMessageID() not monotonically increasing: %d then %d", second, third)
	}
}

// ---------------------------------------------------------------------------
// K. Delete/Suspend/Resume/GetSubscriberStatus return ErrNotImplemented
// ---------------------------------------------------------------------------

func TestDeleteSubscriberNotImplemented(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}
	err := d.DeleteSubscriber(context.Background(), "sub-1")
	if err != types.ErrNotImplemented {
		t.Errorf("DeleteSubscriber() error = %v, want types.ErrNotImplemented", err)
	}
}

func TestSuspendSubscriberNotImplemented(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}
	err := d.SuspendSubscriber(context.Background(), "sub-1")
	if err != types.ErrNotImplemented {
		t.Errorf("SuspendSubscriber() error = %v, want types.ErrNotImplemented", err)
	}
}

func TestResumeSubscriberNotImplemented(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}
	err := d.ResumeSubscriber(context.Background(), "sub-1")
	if err != types.ErrNotImplemented {
		t.Errorf("ResumeSubscriber() error = %v, want types.ErrNotImplemented", err)
	}
}

func TestGetSubscriberStatusNotImplemented(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}
	status, err := d.GetSubscriberStatus(context.Background(), "sub-1")
	if err != types.ErrNotImplemented {
		t.Errorf("GetSubscriberStatus() error = %v, want types.ErrNotImplemented", err)
	}
	if status != nil {
		t.Errorf("GetSubscriberStatus() status = %v, want nil", status)
	}
}

// ---------------------------------------------------------------------------
// ReadMessageContext tests (from original file)
// ---------------------------------------------------------------------------

// mockReader implements io.Reader for testing ReadMessageContext.
// It delivers data in chunks to simulate network reads.
type mockReader struct {
	chunks [][]byte
	index  int
}

func newMockReader(chunks ...string) *mockReader {
	b := make([][]byte, len(chunks))
	for i, c := range chunks {
		b[i] = []byte(c)
	}
	return &mockReader{chunks: b}
}

func (m *mockReader) Read(p []byte) (int, error) {
	if m.index >= len(m.chunks) {
		return 0, io.EOF
	}
	n := copy(p, m.chunks[m.index])
	m.index++
	return n, nil
}

func TestReadMessageContext_NETCONF10(t *testing.T) {
	reader := &netconfReader{
		reader:   newMockReader("<rpc-reply>data</rpc-reply>]]>]]>"),
		useChunk: false,
	}

	got, err := reader.ReadMessageContext(context.Background())
	if err != nil {
		t.Fatalf("ReadMessageContext() error = %v", err)
	}

	want := "<rpc-reply>data</rpc-reply>"
	if string(got) != want {
		t.Errorf("ReadMessageContext() = %q, want %q", string(got), want)
	}
}

func TestReadMessageContext_NETCONF10_MultipleReads(t *testing.T) {
	reader := &netconfReader{
		reader:   newMockReader("<rpc-", "reply/>", "]]>]]>"),
		useChunk: false,
	}

	got, err := reader.ReadMessageContext(context.Background())
	if err != nil {
		t.Fatalf("ReadMessageContext() error = %v", err)
	}

	want := "<rpc-reply/>"
	if string(got) != want {
		t.Errorf("ReadMessageContext() = %q, want %q", string(got), want)
	}
}

func TestReadMessageContext_NETCONF11_Chunked(t *testing.T) {
	reader := &netconfReader{
		reader:   newMockReader("\n#5\nhello\n##\n"),
		useChunk: true,
	}

	got, err := reader.ReadMessageContext(context.Background())
	if err != nil {
		t.Fatalf("ReadMessageContext() error = %v", err)
	}

	if string(got) != "hello" {
		t.Errorf("ReadMessageContext() = %q, want %q", string(got), "hello")
	}
}

func TestReadMessageContext_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	reader := &netconfReader{
		reader:   newMockReader("data that will never be read"),
		useChunk: false,
	}

	_, err := reader.ReadMessageContext(ctx)
	if err == nil {
		t.Fatal("ReadMessageContext() should return error on cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("ReadMessageContext() error = %v, want context.Canceled", err)
	}
}

func TestReadMessageContext_ReadError(t *testing.T) {
	reader := &netconfReader{
		reader:   newMockReader("incomplete data"),
		useChunk: false,
	}

	_, err := reader.ReadMessageContext(context.Background())
	if err == nil {
		t.Fatal("ReadMessageContext() should return error on EOF without EOM")
	}
	if err != io.EOF {
		t.Errorf("ReadMessageContext() error = %v, want io.EOF", err)
	}
}

func TestReadMessageContext_MaxSizeExceeded(t *testing.T) {
	bigData := bytes.Repeat([]byte("x"), maxMessageSize+1)
	reader := &netconfReader{
		reader:   bytes.NewReader(bigData),
		useChunk: false,
	}

	_, err := reader.ReadMessageContext(context.Background())
	if err == nil {
		t.Fatal("ReadMessageContext() should return error when message exceeds max size")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("ReadMessageContext() error = %v, want 'exceeds maximum size'", err)
	}
}

// ---------------------------------------------------------------------------
// L. Disconnect when not connected
// ---------------------------------------------------------------------------

func TestDisconnectWhenNotConnected(t *testing.T) {
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	err := d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() returned error %v, want nil", err)
	}

	// Verify connected is false
	if d.IsConnected() {
		t.Error("IsConnected() should return false after Disconnect")
	}

	// Should be safe to call again
	err = d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("second Disconnect() returned error %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// M. CreateSubscriber when not connected
// ---------------------------------------------------------------------------

func TestCreateSubscriberNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
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

	result, err := d.CreateSubscriber(ctx, subscriber, tier)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

// ---------------------------------------------------------------------------
// N. CreateSubscriber when connected returns result with metadata
// ---------------------------------------------------------------------------

func TestCreateSubscriberConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:    &types.EquipmentConfig{Address: "10.0.0.1"},
		connected: true,
		sessionID: "42",
		capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
		},
	}

	subscriber := &model.Subscriber{
		Name: "sub-test",
		Spec: model.SubscriberSpec{
			ONUSerial:       "ABCD12345678",
			VLAN:            100,
			IPAddress:       "10.0.0.5",
			IPv6Address:     "2001:db8::1",
			DelegatedPrefix: "/56",
			Description:     "Test subscriber",
		},
	}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	result, err := d.CreateSubscriber(ctx, subscriber, tier)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.SubscriberID != "sub-test" {
		t.Errorf("SubscriberID = %q, want %q", result.SubscriberID, "sub-test")
	}
	if result.SessionID != "sess-ABCD12345678" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "sess-ABCD12345678")
	}
	if result.AssignedIP != "10.0.0.5" {
		t.Errorf("AssignedIP = %q, want %q", result.AssignedIP, "10.0.0.5")
	}
	if result.AssignedIPv6 != "2001:db8::1" {
		t.Errorf("AssignedIPv6 = %q, want %q", result.AssignedIPv6, "2001:db8::1")
	}
	if result.AssignedPrefix != "/56" {
		t.Errorf("AssignedPrefix = %q, want %q", result.AssignedPrefix, "/56")
	}
	if result.InterfaceName != "sub-ABCD12345678" {
		t.Errorf("InterfaceName = %q, want %q", result.InterfaceName, "sub-ABCD12345678")
	}
	if result.VLAN != 100 {
		t.Errorf("VLAN = %d, want %d", result.VLAN, 100)
	}
	if result.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if result.Metadata["driver"] != "netconf" {
		t.Errorf("Metadata[driver] = %v, want %q", result.Metadata["driver"], "netconf")
	}
	if result.Metadata["session_id"] != "42" {
		t.Errorf("Metadata[session_id] = %v, want %q", result.Metadata["session_id"], "42")
	}
}

// ---------------------------------------------------------------------------
// O. UpdateSubscriber when not connected (delegates to CreateSubscriber)
// ---------------------------------------------------------------------------

func TestUpdateSubscriberNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	subscriber := &model.Subscriber{
		Name: "sub-test",
		Spec: model.SubscriberSpec{
			ONUSerial: "ABCD12345678",
		},
	}
	tier := &model.ServiceTier{
		Spec: model.ServiceTierSpec{
			BandwidthDown: 100,
			BandwidthUp:   50,
		},
	}

	err := d.UpdateSubscriber(ctx, subscriber, tier)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// P. GetSubscriberStats when not connected
// ---------------------------------------------------------------------------

func TestGetSubscriberStatsNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	_, err := d.GetSubscriberStats(ctx, "sub-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Q. GetSubscriberStats when connected returns placeholder stats
// ---------------------------------------------------------------------------

func TestGetSubscriberStatsConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:    &types.EquipmentConfig{Address: "10.0.0.1"},
		connected: true,
	}

	stats, err := d.GetSubscriberStats(ctx, "sub-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.BytesUp != 0 {
		t.Errorf("BytesUp = %d, want 0", stats.BytesUp)
	}
	if stats.BytesDown != 0 {
		t.Errorf("BytesDown = %d, want 0", stats.BytesDown)
	}
	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if stats.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

// ---------------------------------------------------------------------------
// R. HealthCheck when not connected
// ---------------------------------------------------------------------------

func TestHealthCheckNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	err := d.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// S. RPC when not connected
// ---------------------------------------------------------------------------

func TestRPCNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	_, err := d.RPC(ctx, "<get/>")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// T. Validate without capability
// ---------------------------------------------------------------------------

func TestValidateWithoutCapability(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:       &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{"urn:ietf:params:netconf:base:1.0"},
	}

	err := d.Validate(ctx, "candidate")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "does not support validate") {
		t.Errorf("error %q does not contain 'does not support validate'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// U. NETCONF constants
// ---------------------------------------------------------------------------

func TestNetconfConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{"NetconfBase10", NetconfBase10, "urn:ietf:params:netconf:base:1.0"},
		{"NetconfBase11", NetconfBase11, "urn:ietf:params:netconf:base:1.1"},
		{"NetconfFrameEnd", NetconfFrameEnd, "]]>]]>"},
		{"CapWritableRunning", CapWritableRunning, "urn:ietf:params:netconf:capability:writable-running:1.0"},
		{"CapCandidate", CapCandidate, "urn:ietf:params:netconf:capability:candidate:1.0"},
		{"CapConfirmedCommit", CapConfirmedCommit, "urn:ietf:params:netconf:capability:confirmed-commit:1.0"},
		{"CapRollback", CapRollback, "urn:ietf:params:netconf:capability:rollback-on-error:1.0"},
		{"CapValidate", CapValidate, "urn:ietf:params:netconf:capability:validate:1.0"},
		{"CapStartup", CapStartup, "urn:ietf:params:netconf:capability:startup:1.0"},
		{"CapXPath", CapXPath, "urn:ietf:params:netconf:capability:xpath:1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// V. maxMessageSize constant
// ---------------------------------------------------------------------------

func TestMaxMessageSize(t *testing.T) {
	want := 10 * 1024 * 1024 // 10 MB
	if maxMessageSize != want {
		t.Errorf("maxMessageSize = %d, want %d (10 MB)", maxMessageSize, want)
	}
}

// ---------------------------------------------------------------------------
// W. ReadMessage delegates to ReadMessageContext
// ---------------------------------------------------------------------------

func TestReadMessage(t *testing.T) {
	reader := &netconfReader{
		reader:   newMockReader("<rpc-reply>ok</rpc-reply>]]>]]>"),
		useChunk: false,
	}

	got, err := reader.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}

	want := "<rpc-reply>ok</rpc-reply>"
	if string(got) != want {
		t.Errorf("ReadMessage() = %q, want %q", string(got), want)
	}
}

// ---------------------------------------------------------------------------
// X. netconfReader.Read passthrough
// ---------------------------------------------------------------------------

func TestNetconfReaderRead(t *testing.T) {
	data := "test data"
	reader := &netconfReader{
		reader: strings.NewReader(data),
	}

	buf := make([]byte, 100)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(buf[:n]) != data {
		t.Errorf("Read() = %q, want %q", string(buf[:n]), data)
	}
}

// ---------------------------------------------------------------------------
// Y. UpdateSubscriber when connected returns nil (delegates to CreateSubscriber)
// ---------------------------------------------------------------------------

func TestUpdateSubscriberConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config:    &types.EquipmentConfig{Address: "10.0.0.1"},
		connected: true,
		sessionID: "42",
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

	err := d.UpdateSubscriber(ctx, subscriber, tier)
	if err != nil {
		t.Errorf("UpdateSubscriber() returned error %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Z. NETCONFExecutor interface compliance
// ---------------------------------------------------------------------------

func TestDriverImplementsNETCONFExecutor(t *testing.T) {
	d := &Driver{config: &types.EquipmentConfig{Address: "10.0.0.1"}}
	var _ NETCONFExecutor = d
}

// ---------------------------------------------------------------------------
// AA. Get, GetConfig, Commit, DiscardChanges, Lock, Unlock when not connected
// (They all delegate to RPC which checks connected)
// ---------------------------------------------------------------------------

func TestNetconfOperationsNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
	}

	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Get when not connected",
			fn: func() error {
				_, err := d.Get(ctx, "")
				return err
			},
		},
		{
			name: "Get with filter when not connected",
			fn: func() error {
				_, err := d.Get(ctx, "<interfaces/>")
				return err
			},
		},
		{
			name: "GetConfig when not connected",
			fn: func() error {
				_, err := d.GetConfig(ctx, "", "")
				return err
			},
		},
		{
			name: "GetConfig with source and filter when not connected",
			fn: func() error {
				_, err := d.GetConfig(ctx, "running", "<interfaces/>")
				return err
			},
		},
		{
			name: "Commit when not connected",
			fn: func() error {
				return d.Commit(ctx)
			},
		},
		{
			name: "DiscardChanges when not connected",
			fn: func() error {
				return d.DiscardChanges(ctx)
			},
		},
		{
			name: "Lock when not connected",
			fn: func() error {
				return d.Lock(ctx, "")
			},
		},
		{
			name: "Lock with target when not connected",
			fn: func() error {
				return d.Lock(ctx, "candidate")
			},
		},
		{
			name: "Unlock when not connected",
			fn: func() error {
				return d.Unlock(ctx, "")
			},
		},
		{
			name: "Unlock with target when not connected",
			fn: func() error {
				return d.Unlock(ctx, "candidate")
			},
		},
		{
			name: "EditConfig when not connected",
			fn: func() error {
				return d.EditConfig(ctx, "", "<config/>")
			},
		},
		{
			name: "EditConfig with options when not connected",
			fn: func() error {
				return d.EditConfig(ctx, "running", "<config/>", WithMerge(), WithRollbackOnError())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "not connected") {
				t.Errorf("error %q does not contain 'not connected'", err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AB. Validate with capability but not connected
// ---------------------------------------------------------------------------

func TestValidateWithCapabilityButNotConnected(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:capability:validate:1.0",
		},
	}

	err := d.Validate(ctx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// It should pass the capability check but fail on RPC due to not connected
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AC. Disconnect when connected=true but session/sshClient are nil
// ---------------------------------------------------------------------------

func TestDisconnectConnectedButNilSessionAndClient(t *testing.T) {
	d := &Driver{
		config:    &types.EquipmentConfig{Address: "10.0.0.1"},
		connected: true,
		stdin:     &netconfWriter{writer: &bytes.Buffer{}, useChunk: false},
	}

	err := d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() returned error %v, want nil", err)
	}
	if d.IsConnected() {
		t.Error("should not be connected after Disconnect")
	}
}

// ---------------------------------------------------------------------------
// AD. EditConfig targeting candidate (has CapCandidate)
// ---------------------------------------------------------------------------

func TestEditConfigTargetDefault(t *testing.T) {
	ctx := context.Background()

	// Driver with candidate capability but not connected
	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:capability:candidate:1.0",
		},
	}

	err := d.EditConfig(ctx, "", "<config/>")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Should fail because not connected, but the candidate target should be selected
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AE. EditConfig with test-then-set option and validate capability
// ---------------------------------------------------------------------------

func TestEditConfigWithTestOption(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:capability:validate:1.0",
		},
	}

	err := d.EditConfig(ctx, "running", "<config/>", WithTestThenSet())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Not connected error is expected
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AF. Validate with empty source defaults to candidate
// ---------------------------------------------------------------------------

func TestValidateWithEmptySource(t *testing.T) {
	ctx := context.Background()

	d := &Driver{
		config: &types.EquipmentConfig{Address: "10.0.0.1"},
		capabilities: []string{
			"urn:ietf:params:netconf:capability:validate:1.0",
		},
	}

	err := d.Validate(ctx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Has validate capability, but RPC fails because not connected
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q does not contain 'not connected'", err.Error())
	}
}
