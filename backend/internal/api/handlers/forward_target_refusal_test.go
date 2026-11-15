package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

// Every refusal cause must get the same text and error code, so the response
// does not reveal which rule applied.
func TestRespondForwardTargetRefused_OneTextForEveryCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	causes := []error{
		services.ErrOrthrusListenerTarget,
		services.ErrAgentForwardTarget,
		services.ErrForwardTargetAddress,
		fmt.Errorf("wrapped: %w", services.ErrForwardTargetAddress),
	}
	for _, cause := range causes {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		require.True(t, respondForwardTargetRefused(c, cause), cause.Error())
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.JSONEq(t, `{"error":"This forward target is not allowed.","error_code":"forward_target_refused"}`, rec.Body.String())
	}
}

func TestRespondForwardTargetRefused_IgnoresOtherErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	assert.False(t, respondForwardTargetRefused(c, errors.New("database is locked")))
	assert.Equal(t, http.StatusOK, rec.Code, "nothing is written for other errors")
}
