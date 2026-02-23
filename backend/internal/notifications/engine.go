package notifications

import "context"

const (
	EngineLegacy   = "legacy"
	EngineNotifyV1 = "notify_v1"
)

type DispatchRequest struct {
	ProviderID string
	Type       string
	URL        string
	Title      string
	Message    string
	Data       map[string]any
}

type DeliveryEngine interface {
	Name() string
	Send(ctx context.Context, req DispatchRequest) error
	Test(ctx context.Context, req DispatchRequest) error
}
