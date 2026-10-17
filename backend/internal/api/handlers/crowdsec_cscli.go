package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

const (
	// cscliListTimeout bounds the cscli commands that read (decisions list, alerts list). With the local API
	// down cscli retries for minutes before it exits, which would otherwise hold the request open.
	cscliListTimeout = 30 * time.Second
	// cscliChangeTimeout bounds the cscli commands that add or delete a decision.
	cscliChangeTimeout = 15 * time.Second
	// registerScriptTimeoutDefault bounds the bouncer registration script, which runs several cscli commands.
	registerScriptTimeoutDefault = 30 * time.Second

	// msgCSCLIFailed is the soft error of a listing when cscli failed or printed more than the limits allow.
	msgCSCLIFailed       = "cscli not available or failed"
	msgDecisionsTimedOut = "cscli decisions list timed out; the CrowdSec local API may be stopped"
	msgAlertsTimedOut    = "cscli alerts list timed out; the CrowdSec local API may be stopped"

	codeCSCLITimeout = "cscli_timeout"
	msgCSCLITimeout  = "CrowdSec did not answer in time"
)

// runCSCLI runs one cscli command under its own deadline. It puts the configuration file the engine reads
// (config/config.yaml, else config.yaml) in front of the arguments when one exists. A command that ran out of
// time returns an error that wraps context.DeadlineExceeded, whether the process was killed or reported the
// context error itself.
func (h *CrowdsecHandler) runCSCLI(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	if configPath := crowdsec.FindConfigFile(h.DataDir); configPath != "" {
		args = append([]string{"-c", configPath}, args...)
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := h.CmdExec.Execute(runCtx, "cscli", args...)
	if err != nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("cscli did not finish in time: %w: %w", context.DeadlineExceeded, err)
	}
	return out, err
}

// isCSCLITimeout reports whether a failed cscli command ran out of time. runCSCLI marks such an error; ctx is
// the caller's context, whose own expiry counts as well.
func isCSCLITimeout(ctx context.Context, err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)
}

// respondCSCLITimeout writes the coded 504 for a cscli command that ran out of time.
func respondCSCLITimeout(c *gin.Context) {
	respondCoded(c, http.StatusGatewayTimeout, codeCSCLITimeout, msgCSCLITimeout, nil)
}
