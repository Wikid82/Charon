package leash

import (
	"context"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
)

const (
	heartbeatInterval   = 30 * time.Second
	heartbeatTimeout    = 10 * time.Second
	maxFailedHeartbeats = 3
)

type heartbeat struct {
	session *yamux.Session
	log     *logrus.Logger
}

func newHeartbeat(session *yamux.Session, log *logrus.Logger) *heartbeat {
	return &heartbeat{session: session, log: log}
}

// Run sends yamux pings on the interval and closes the session after
// maxFailedHeartbeats consecutive failures to trigger a reconnect.
func (h *heartbeat) Run(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	failures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingErr := make(chan error, 1)
			go func() {
				_, err := h.session.Ping()
				pingErr <- err
			}()

			var err error
			select {
			case err = <-pingErr:
			case <-time.After(heartbeatTimeout):
				err = context.DeadlineExceeded
			}

			if err != nil {
				failures++
				h.log.WithError(err).WithField("failures", failures).Warn("leash: heartbeat ping failed")
				if failures >= maxFailedHeartbeats {
					h.log.Error("leash: too many consecutive heartbeat failures, closing session")
					_ = h.session.Close()
					return
				}
			} else {
				failures = 0
			}
		}
	}
}
