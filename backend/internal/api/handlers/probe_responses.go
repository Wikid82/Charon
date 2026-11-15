package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Fixed texts for destination checks and connection tests. They name no address,
// port or rule, so a response never reveals this machine's reserved ports or the
// destination policy.
const (
	destinationRefusedText   = "This destination is not allowed."
	destinationRefusedCode   = "destination_refused"
	connectionTestFailedText = "Could not connect to the target."
	connectionTestErrorText  = "Could not run the connection test."
)

// respondDestinationRefused writes the 400 for a destination the probe policy refuses.
func respondDestinationRefused(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": destinationRefusedText, "error_code": destinationRefusedCode})
}
