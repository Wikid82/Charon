package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// maxCrowdsecJSONBodyBytes caps the request body of the small CrowdSec JSON endpoints (ban, whitelist,
	// enrollment, preset pull and apply, acquisition). Endpoints that carry file content set their own cap.
	maxCrowdsecJSONBodyBytes = 64 << 10

	codeRequestTooLarge = "request_too_large"
	msgRequestTooLarge  = "the request body is too large"
)

// bindSmallJSON reads the request body as JSON into obj, refusing a body above maxCrowdsecJSONBodyBytes with
// 413 request_too_large. For any other read or decode failure it calls onInvalid, which sends the endpoint's own
// 400 answer. It returns false once a response has been sent.
func bindSmallJSON(c *gin.Context, obj any, onInvalid func()) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCrowdsecJSONBodyBytes)
	err := c.ShouldBindJSON(obj)
	if err == nil {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		respondCoded(c, http.StatusRequestEntityTooLarge, codeRequestTooLarge, msgRequestTooLarge, nil)
		return false
	}
	onInvalid()
	return false
}
