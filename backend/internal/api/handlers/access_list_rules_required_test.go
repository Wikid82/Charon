package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAccessListHandler_RulesRequiredErrorCode(t *testing.T) {
	router, db := setupAccessListTestRouter(t)

	t.Run("create refuses an enabled allow-list without rules", func(t *testing.T) {
		body := []byte(`{"name":"wl","type":"whitelist","enabled":true,"ip_rules":""}`)
		req := httptest.NewRequest(http.MethodPost, "/access-lists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "access_list_rules_required", resp["error_code"])
		require.NotEmpty(t, resp["error"])
	})

	t.Run("create keeps a block-list without rules", func(t *testing.T) {
		body := []byte(`{"name":"bl","type":"blacklist","enabled":true,"ip_rules":""}`)
		req := httptest.NewRequest(http.MethodPost, "/access-lists", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("update refuses enabling an allow-list without rules", func(t *testing.T) {
		acl := models.AccessList{UUID: "rules-required-update", Name: "wl", Type: "whitelist", Enabled: false, IPRules: `[{"cidr":"10.0.0.1"}]`}
		require.NoError(t, db.Create(&acl).Error)

		body := []byte(`{"name":"wl","type":"whitelist","enabled":true,"ip_rules":""}`)
		req := httptest.NewRequest(http.MethodPut, "/access-lists/"+acl.UUID, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "access_list_rules_required", resp["error_code"])
	})
}
