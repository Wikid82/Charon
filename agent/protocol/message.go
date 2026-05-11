// Package protocol defines the message framing types for the Orthrus Leash protocol.
//
// The wire protocol uses yamux for multiplexing; each data stream is identified by the
// first byte written on the stream. The control channel uses yamux's built-in Ping
// mechanism for heartbeats rather than a dedicated user-level stream.
package protocol

// MessageType identifies the type of a yamux stream or control message.
type MessageType uint8

const (
	// TypePing is a control message requesting a pong response.
	TypePing MessageType = 0x00

	// TypePong is a control message response to a ping.
	TypePong MessageType = 0x01

	// TypePortForward indicates a TCP port-forward data stream.
	// The stream header is: 2-byte big-endian uint16 length, then the target address bytes.
	TypePortForward MessageType = 0x02

	// TypeDockerSocket indicates a Docker socket proxy data stream.
	// The stream carries a raw HTTP request followed by the server's response.
	TypeDockerSocket MessageType = 0x03

	// TypeError indicates an error condition on a stream.
	TypeError MessageType = 0x04
)

// Message is a framed message exchanged on the Orthrus Leash control channel.
// Data streams use raw yamux streams with a leading MessageType byte; they do
// not use this struct on the wire.
type Message struct {
	Type     MessageType
	StreamID uint32
	Payload  []byte
}
