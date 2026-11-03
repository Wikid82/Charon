package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// errRegistrationRefused is returned by a registration that starts after shutdown has begun.
var errRegistrationRefused = errors.New("bouncer registration refused: shutting down")

// beginRegistration counts one registration in flight. It returns false once WaitRegistrations has closed the
// gate; the caller must then not touch the bouncer and must not call endRegistration.
func (h *CrowdsecHandler) beginRegistration() bool {
	h.registrationGate.Lock()
	defer h.registrationGate.Unlock()
	if h.registrationsClosed {
		return false
	}
	h.registrations.Add(1)
	return true
}

// endRegistration ends a registration counted by beginRegistration.
func (h *CrowdsecHandler) endRegistration() {
	h.registrations.Done()
}

// WaitRegistrations closes the registration gate, so no new bouncer swap can start, and waits until the swaps in
// flight have finished or ctx ends. It is called once, at shutdown. The swaps run on a context that the shutdown
// cancel does not end, so this wait is what lets them finish before the process exits.
func (h *CrowdsecHandler) WaitRegistrations(ctx context.Context) error {
	h.registrationGate.Lock()
	h.registrationsClosed = true
	h.registrationGate.Unlock()

	done := make(chan struct{})
	go func() {
		h.registrations.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for bouncer registrations: %w", ctx.Err())
	}
}

const (
	codeShuttingDown = "shutting_down"
	msgShuttingDown  = "CrowdSec is shutting down; try again after Charon restarts"
)

// respondShuttingDown answers 503 for a registration that arrives after shutdown began.
func respondShuttingDown(c *gin.Context) {
	respondCoded(c, http.StatusServiceUnavailable, codeShuttingDown, msgShuttingDown, nil)
}
