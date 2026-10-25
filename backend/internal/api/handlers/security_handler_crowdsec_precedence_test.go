package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// crowdSecAbsent marks a settings key that is not written at all in a precedence case.
const crowdSecAbsent = "<absent>"

// crowdSecPrecedenceCase mirrors the case list in the caddy and services test packages.
type crowdSecPrecedenceCase struct {
	enabled     string
	mode        string
	row         string
	cerberusOff bool
	wantOn      bool
}

func crowdSecWantOn(enabled, mode, row string) bool {
	if enabled != crowdSecAbsent && enabled != "" {
		return enabled == "true"
	}
	if mode != crowdSecAbsent && mode != "" {
		return mode == "local"
	}
	return row == "local"
}

func crowdSecPrecedenceCases() []crowdSecPrecedenceCase {
	var cases []crowdSecPrecedenceCase
	for _, enabled := range []string{crowdSecAbsent, "", "true", "false"} {
		for _, mode := range []string{crowdSecAbsent, "", "local", "disabled", "remote"} {
			for _, row := range []string{"local", "disabled"} {
				cases = append(cases, crowdSecPrecedenceCase{
					enabled: enabled,
					mode:    mode,
					row:     row,
					wantOn:  crowdSecWantOn(enabled, mode, row),
				})
			}
		}
	}
	cases = append(cases, crowdSecPrecedenceCase{enabled: "true", row: "local", cerberusOff: true, wantOn: true})
	return cases
}

func TestSecurityHandler_GetStatus_CrowdSecPrecedenceMatrix(t *testing.T) {
	for _, tc := range crowdSecPrecedenceCases() {
		name := "enabled=" + tc.enabled + "/mode=" + tc.mode + "/row=" + tc.row
		if tc.cerberusOff {
			name += "/cerberus=off"
		}
		t.Run(name, func(t *testing.T) {
			db := setupTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.Setting{}, &models.CaddyConfig{}))
			for key, value := range map[string]string{
				"security.crowdsec.enabled": tc.enabled,
				"security.crowdsec.mode":    tc.mode,
			} {
				if value == crowdSecAbsent {
					continue
				}
				require.NoError(t, db.Create(&models.Setting{Key: key, Value: value, Category: "security", Type: "string"}).Error)
			}
			require.NoError(t, db.Create(&models.SecurityConfig{
				Name:         models.DefaultSecurityConfigName,
				Enabled:      !tc.cerberusOff,
				CrowdSecMode: tc.row,
			}).Error)

			handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
			t.Cleanup(handler.Close)
			router := gin.New()
			router.GET("/security/status", handler.GetStatus)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/security/status", http.NoBody)
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			var resp struct {
				CrowdSec struct {
					Mode    string `json:"mode"`
					Enabled bool   `json:"enabled"`
				} `json:"crowdsec"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			// The status endpoint reports CrowdSec off whenever the global switch is off, as the proxy does.
			effectiveOn := tc.wantOn && !tc.cerberusOff
			require.Equal(t, effectiveOn, resp.CrowdSec.Enabled)
			wantMode := "disabled"
			if effectiveOn {
				wantMode = "local"
			}
			require.Equal(t, wantMode, resp.CrowdSec.Mode)
		})
	}
}
