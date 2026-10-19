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
	// bouncerCheckTimeout bounds the bouncer list and delete commands; bouncerRegisterTimeout bounds the add.
	bouncerCheckTimeout    = 5 * time.Second
	bouncerRegisterTimeout = 10 * time.Second
	// registerScriptTimeoutDefault bounds the bouncer registration script, which runs several cscli commands.
	registerScriptTimeoutDefault = 30 * time.Second

	// msgCSCLIFailed is the soft error of a listing when cscli failed or printed more than the limits allow.
	msgCSCLIFailed       = "cscli not available or failed"
	msgDecisionsTimedOut = "cscli decisions list timed out; the CrowdSec local API may be stopped"
	msgAlertsTimedOut    = "cscli alerts list timed out; the CrowdSec local API may be stopped"

	codeCSCLITimeout        = "cscli_timeout"
	codeCSCLIOutputTooLarge = "cscli_output_too_large"
	msgCSCLITimeout         = "CrowdSec did not answer in time"
)

// runCSCLI runs one cscli command under its own deadline. It puts the configuration file the engine reads
// (config/config.yaml, else config.yaml) in front of the arguments when one exists. A command that ran out of
// time returns an error that wraps context.DeadlineExceeded, whether the process was killed or reported the
// context error itself.
func (h *CrowdsecHandler) runCSCLI(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := crowdsec.RunCSCLI(runCtx, h.CmdExec, crowdsec.FindConfigFile(h.DataDir), args...)
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

// softListError is the text and stable code a listing answers with, next to an empty list and a 200, when
// cscli failed. The text stays as the fallback for clients that do not know the code.
type softListError struct {
	msg  string
	code string
}

// classifySoftListError maps a failed cscli listing to its soft error. timedOutMsg is the text for the
// listing that ran out of time; ctx is the caller's context, whose own expiry counts as a timeout.
func classifySoftListError(ctx context.Context, err error, timedOutMsg string) *softListError {
	switch {
	case isCSCLITimeout(ctx, err):
		return &softListError{msg: timedOutMsg, code: codeCSCLITimeout}
	case errors.Is(err, crowdsec.ErrOutputLimit), errors.Is(err, crowdsec.ErrCSCLIOutputTooLarge):
		return &softListError{msg: msgCSCLIFailed, code: codeCSCLIOutputTooLarge}
	default:
		return &softListError{msg: msgCSCLIFailed, code: codeCSCLIUnavailable}
	}
}

// apply adds the error and its code to a listing response.
func (e softListError) apply(body gin.H) {
	body["error"] = e.msg
	body["error_code"] = e.code
}
