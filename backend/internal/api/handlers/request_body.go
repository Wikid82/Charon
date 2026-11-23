package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	// maxSmallJSONBodyBytes caps the request body of the small JSON endpoints (CrowdSec ban, whitelist,
	// enrollment, preset pull and apply, acquisition). Endpoints that carry file content set their own cap.
	maxSmallJSONBodyBytes = 64 << 10
	// maxHostRequestBodyBytes caps proxy and redirection host requests.
	maxHostRequestBodyBytes = 1 << 20
	// maxImportBodyBytes caps Caddyfile import requests.
	maxImportBodyBytes = 4 << 20
	// maxJSONImportBodyBytes caps JSON and Nginx Proxy Manager import requests.
	maxJSONImportBodyBytes = 8 << 20

	codeRequestTooLarge = "request_too_large"
	msgRequestTooLarge  = "the request body is too large"
)

// limitRequestBody caps the request body of the routes it guards at n bytes. A read past the cap fails with
// *http.MaxBytesError, which the bind helpers answer with 413 request_too_large.
func limitRequestBody(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		wrapRequestBody(c, n)
		c.Next()
	}
}

// wrapRequestBody replaces the request body with one that fails past n bytes.
func wrapRequestBody(c *gin.Context, n int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
}

// bindSmallJSON reads the request body as JSON into obj, refusing a body above maxSmallJSONBodyBytes with
// 413 request_too_large. For any other read or decode failure it calls onInvalid, which sends the endpoint's own
// 400 answer. It returns false once a response has been sent.
func bindSmallJSON(c *gin.Context, obj any, onInvalid func()) bool {
	wrapRequestBody(c, maxSmallJSONBodyBytes)
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
