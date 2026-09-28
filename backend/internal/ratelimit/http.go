package ratelimit

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// TooManyRequestsMessage is the single, generic 429 body used by every
// throttle. It never echoes request data.
const TooManyRequestsMessage = "Too many requests. Please wait before trying again."

// RetryAfterSeconds converts a wait into Retry-After seconds: rounded to the
// millisecond (absorbing float noise), then rounded up, minimum 1.
func RetryAfterSeconds(d time.Duration) int {
	secs := int(math.Ceil(d.Round(time.Millisecond).Seconds()))
	return max(secs, 1)
}

// Reject aborts the request with 429, a Retry-After header and the generic body.
func Reject(c *gin.Context, d Decision) {
	c.Header("Retry-After", strconv.Itoa(RetryAfterSeconds(d.RetryAfter)))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": TooManyRequestsMessage})
}
