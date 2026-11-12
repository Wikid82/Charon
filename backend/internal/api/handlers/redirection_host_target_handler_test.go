package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectionHostHandler_Create_LiteralBraceTarget_400WithCode(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["target_url"] = "https://newblog.example.com/{env.SECRET_VALUE}"

	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	require.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "redirect_target_invalid", body["error_code"])
	assert.NotContains(t, body["error"], "SECRET_VALUE", "the response must not echo the submitted target")
}

func TestRedirectionHostHandler_Update_LiteralBraceTarget_400WithCode(t *testing.T) {
	router, db := setupRedirectionHostHandlerRouter(t)
	createRes := doRequest(router, http.MethodPost, "/redirection-hosts", validRedirectionHostPayload())
	require.Equal(t, http.StatusCreated, createRes.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(createRes.Body.Bytes(), &created))
	uuid, _ := created["uuid"].(string)
	require.NotEmpty(t, uuid)

	w := doRequest(router, http.MethodPut, "/redirection-hosts/"+uuid, map[string]any{"target_url": "https://newblog.example.com/}"})
	require.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "redirect_target_invalid", body["error_code"])

	// The stored row is unchanged by the refused update.
	var stored struct{ TargetURL string }
	require.NoError(t, db.Table("redirection_hosts").Select("target_url").Where("uuid = ?", uuid).Scan(&stored).Error)
	assert.Equal(t, "https://newblog.example.com", stored.TargetURL)
}

func TestRedirectionHostHandler_Create_PercentEncodedTarget_201(t *testing.T) {
	router, _ := setupRedirectionHostHandlerRouter(t)
	payload := validRedirectionHostPayload()
	payload["target_url"] = "https://newblog.example.com/%7Benv.X%7D"

	w := doRequest(router, http.MethodPost, "/redirection-hosts", payload)
	assert.Equal(t, http.StatusCreated, w.Code)
}
