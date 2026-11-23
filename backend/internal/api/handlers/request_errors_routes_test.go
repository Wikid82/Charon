package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Requests the host routes cannot bind get the plain answer on the real routes: no Go name, type or parser text.
func TestHostRoutesAnswerUnreadableRequestsWithThePlainText(t *testing.T) {
	t.Parallel()
	db := openHostRestoreDB(t)
	host := seedHost(t, db, "route-plain")
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})
	forbidden := []string{"Key:", "Error:Field", "struct", "json:", "models.", "cannot unmarshal", "offset", "invalid character"}

	cases := []struct {
		name, method, path, body, check string
	}{
		{"create with a text port", http.MethodPost, "/api/v1/proxy-hosts", `{"forward_port":"abc"}`, "Check: forward_port."},
		{"create with broken JSON", http.MethodPost, "/api/v1/proxy-hosts", `{"name":`, ""},
		{"update with a list", http.MethodPut, "/api/v1/proxy-hosts/" + host.UUID, `[1]`, ""},
		{"bulk acl with a text list", http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", `{"host_uuids":"x"}`, "Check: host_uuids."},
		{"redirection create with broken JSON", http.MethodPost, "/api/v1/redirection-hosts", `{"name":`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := f.doRaw(t, tc.method, tc.path, tc.body)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			answer := decodeBody(t, w)
			assert.Equal(t, codeInvalidRequest, answer["error_code"])
			if tc.check != "" {
				assert.Contains(t, answer["error"], tc.check)
			} else {
				assert.Equal(t, msgInvalidRequest, answer["error"])
			}
			for _, word := range forbidden {
				assert.NotContains(t, w.Body.String(), word)
			}
		})
	}
}

// The validation answer for a host body never names a Go field or type, even when the body is a list.
func TestHostBodyAnswersNameNoGoDetail(t *testing.T) {
	t.Parallel()
	db := openHostRestoreDB(t)
	f := newHostRestoreFixture(t, db, string(models.RoleAdmin), &refusingApplier{})
	w := f.doRaw(t, http.MethodPost, "/api/v1/proxy-hosts", strings.Repeat("[", 3))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "Go")
}
