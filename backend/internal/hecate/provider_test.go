package hecate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTunnelStateConstants(t *testing.T) {
	assert.Equal(t, TunnelState("connected"), TunnelStateConnected)
	assert.Equal(t, TunnelState("connecting"), TunnelStateConnecting)
	assert.Equal(t, TunnelState("error"), TunnelStateError)
	assert.Equal(t, TunnelState("stopped"), TunnelStateStopped)
}

func TestMockProvider_ImplementsInterface(t *testing.T) {
	p := newMockProvider("test")
	var _ TunnelProvider = p // compile-time check

	assert.Equal(t, "test", p.Name())
	assert.Equal(t, TunnelStateStopped, p.Status())
	assert.Equal(t, "127.0.0.1:9999", p.GetAddress())

	assert.NoError(t, p.Start(context.Background()))
	assert.Equal(t, TunnelStateConnected, p.Status())

	assert.NoError(t, p.Stop())
	assert.Equal(t, TunnelStateStopped, p.Status())
}

func TestTunnelStatus_Fields(t *testing.T) {
	s := TunnelStatus{
		UUID:          "abc",
		Name:          "my-tunnel",
		Provider:      "cloudflare",
		State:         TunnelStateConnected,
		UptimeSeconds: 120,
		LastError:     "",
	}
	assert.Equal(t, "abc", s.UUID)
	assert.Equal(t, "my-tunnel", s.Name)
	assert.Equal(t, "cloudflare", s.Provider)
	assert.Equal(t, TunnelStateConnected, s.State)
	assert.Equal(t, int64(120), s.UptimeSeconds)
	assert.Equal(t, "", s.LastError)
}
