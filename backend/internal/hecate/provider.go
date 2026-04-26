package hecate

import "context"

// TunnelState represents the lifecycle state of a tunnel provider.
type TunnelState string

const (
	TunnelStateConnected  TunnelState = "connected"
	TunnelStateConnecting TunnelState = "connecting"
	TunnelStateError      TunnelState = "error"
	TunnelStateStopped    TunnelState = "stopped"
)

// TunnelStatus is a point-in-time snapshot of a managed tunnel's state.
type TunnelStatus struct {
	UUID          string      `json:"uuid"`
	Name          string      `json:"name"`
	Provider      string      `json:"provider"`
	State         TunnelState `json:"state"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	LastError     string      `json:"last_error,omitempty"`
}

// TunnelProvider is the interface that every external tunnel backend must satisfy.
type TunnelProvider interface {
	Name() string
	Status() TunnelState
	Start(ctx context.Context) error
	Stop() error
	GetAddress() string
}
