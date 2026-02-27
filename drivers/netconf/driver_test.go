package netconf

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

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
	// NETCONF 1.0 uses ]]>]]> as end-of-message
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
	// Data arrives in multiple reads
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
	// EOF without EOM marker
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
	// Create a reader that produces data larger than maxMessageSize
	// We use a custom reader that keeps producing data
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
