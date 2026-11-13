package routes

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

const notificationAlertsPath = "/api/v1/notifications/alerts"

func TestRegister_NotificationAlerts_UnauthenticatedRejected(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))

	w := app.do(http.MethodGet, notificationAlertsPath, "", "", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegister_NotificationAlerts_PassthroughRejected(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))
	_, token := app.createUser(models.RolePassthrough)

	w := app.do(http.MethodGet, notificationAlertsPath, token, "", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRegister_NotificationAlerts_ManagementUserGetsAlerts pins that GET resolves to the alerts handler for viewers and
// admins. With no alert sources registered, the answer is an empty list.
func TestRegister_NotificationAlerts_ManagementUserGetsAlerts(t *testing.T) {
	app := newThrottledApp(t, withAuthBudget(50, 50))

	for _, role := range []models.UserRole{models.RoleUser, models.RoleAdmin} {
		_, token := app.createUser(role)

		w := app.do(http.MethodGet, notificationAlertsPath, token, "", nil)

		require.Equal(t, http.StatusOK, w.Code, "role %s body: %s", role, w.Body.String())
		var body map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.JSONEq(t, `[]`, string(body["alerts"]), "role %s", role)
	}
}
