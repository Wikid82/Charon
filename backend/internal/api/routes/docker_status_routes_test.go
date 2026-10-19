package routes

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

const dockerStatusPath = "/api/v1/docker/status"

func TestRegister_DockerStatus_UnauthenticatedRejected(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))

	w := app.do(http.MethodGet, dockerStatusPath, "", "", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegister_DockerStatus_PassthroughRejected(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))
	_, token := app.createUser(models.RolePassthrough)

	w := app.do(http.MethodGet, dockerStatusPath, token, "", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRegister_DockerStatus_ManagementUserGetsStatus(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))

	for _, role := range []models.UserRole{models.RoleUser, models.RoleAdmin} {
		_, token := app.createUser(role)

		w := app.do(http.MethodGet, dockerStatusPath, token, "", nil)

		require.Equal(t, http.StatusOK, w.Code, "role %s body: %s", role, w.Body.String())
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Contains(t, body, "available")
		assert.Contains(t, body, "reason")
		assert.Contains(t, body, "message")
	}
}
