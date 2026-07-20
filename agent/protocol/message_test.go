package protocol_test

import (
	"testing"

	"github.com/Wikid82/charon/agent/protocol"
)

// TestMessageTypeConstants guards the documented, distinct byte values of
// the MessageType constants against an accidental future change. protocol
// has no behavioral logic to test (pure constant/type declarations), so
// this is added mainly so `go test ./...` doesn't report "[no test files]"
// for a package now in-scope for the agent coverage gate (Section 3.5 of
// the Orthrus spec), and to catch a wire-protocol-breaking constant-value
// change: these bytes are the first byte on every yamux stream and must
// stay stable across agent/backend versions.
func TestMessageTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  protocol.MessageType
		want protocol.MessageType
	}{
		{"TypePing", protocol.TypePing, 0x00},
		{"TypePong", protocol.TypePong, 0x01},
		{"TypePortForward", protocol.TypePortForward, 0x02},
		{"TypeDockerSocket", protocol.TypeDockerSocket, 0x03},
		{"TypeError", protocol.TypeError, 0x04},
	}

	seen := make(map[protocol.MessageType]string, len(tests))
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = 0x%02x, want 0x%02x", tc.name, tc.got, tc.want)
		}
		if existing, dup := seen[tc.got]; dup {
			t.Errorf("%s shares byte value 0x%02x with %s -- MessageType values must be distinct", tc.name, tc.got, existing)
		}
		seen[tc.got] = tc.name
	}
}

// TestMessage_FieldAssignment confirms Message's fields round-trip as a
// plain struct (no custom marshaling logic to bypass) -- the minimal
// behavior this pure-data type has.
func TestMessage_FieldAssignment(t *testing.T) {
	msg := protocol.Message{
		Type:     protocol.TypeDockerSocket,
		StreamID: 42,
		Payload:  []byte("hello"),
	}

	if msg.Type != protocol.TypeDockerSocket {
		t.Errorf("Type = 0x%02x, want 0x%02x", msg.Type, protocol.TypeDockerSocket)
	}
	if msg.StreamID != 42 {
		t.Errorf("StreamID = %d, want 42", msg.StreamID)
	}
	if string(msg.Payload) != "hello" {
		t.Errorf("Payload = %q, want %q", string(msg.Payload), "hello")
	}
}
