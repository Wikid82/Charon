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
	if respondIfBodyTooLarge(c, err) {
		return false
	}
	onInvalid()
	return false
}

// respondIfBodyTooLarge answers 413 request_too_large when err is a request body passing its limit, and reports
// whether it did. The request body must already be wrapped in http.MaxBytesReader.
func respondIfBodyTooLarge(c *gin.Context, err error) bool {
	var tooLarge *http.MaxBytesError
	if !errors.As(err, &tooLarge) {
		return false
	}
	respondCoded(c, http.StatusRequestEntityTooLarge, codeRequestTooLarge, msgRequestTooLarge, nil)
	return true
}
