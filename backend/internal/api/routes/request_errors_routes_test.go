package routes_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// bindHandlerMethods lists every handler method that reads its request with bindJSON, bindQuery or bindSmallJSON, as
// "Type.Method". TestBindHandlerTableMatchesTheSource fails when a bind call site is added or removed without this list
// changing, so a new bind site is always walked by TestRegister_EveryBindSiteAnswersAMalformedRequestPlainly.
var bindHandlerMethods = []string{
	"AccessListHandler.Create", "AccessListHandler.TestIP", "AccessListHandler.Update",
	"AuthHandler.ChangePassword", "AuthHandler.Login",
	"BackupHandler.UpdateSettings",
	"BackupRemoteHandler.Create", "BackupRemoteHandler.TestDraft", "BackupRemoteHandler.Update",
	"CredentialHandler.Create", "CredentialHandler.Update",
	"CrowdsecHandler.AddWhitelist", "CrowdsecHandler.ApplyPreset", "CrowdsecHandler.BanIP", "CrowdsecHandler.ConsoleEnroll",
	"CrowdsecHandler.PullPreset", "CrowdsecHandler.UpdateAcquisitionConfig",
	"CustomThemeHandler.CreateTheme", "CustomThemeHandler.UpdateTheme",
	"DNSProviderHandler.Create", "DNSProviderHandler.TestCredentials", "DNSProviderHandler.Update",
	"DomainHandler.Create",
	"EmergencyHandler.GenerateToken", "EmergencyHandler.UpdateTokenExpiration",
	"FeatureFlagsHandler.UpdateFlags",
	"HecateHandler.Create", "HecateHandler.RotateCredentials", "HecateHandler.Update",
	"ImportHandler.Commit", "ImportHandler.DetectImports", "ImportHandler.Upload", "ImportHandler.UploadMulti",
	"JSONImportHandler.Cancel", "JSONImportHandler.Commit", "JSONImportHandler.Upload",
	"LogsHandler.Read",
	"NPMImportHandler.Cancel", "NPMImportHandler.Commit", "NPMImportHandler.Upload",
	"NotificationTemplateHandler.Create", "NotificationTemplateHandler.Preview", "NotificationTemplateHandler.Update",
	"OrthrusHandler.Patch", "OrthrusHandler.Provision",
	"ProxyGroupHandler.Create", "ProxyGroupHandler.Update",
	"ProxyHostHandler.BulkUpdateACL", "ProxyHostHandler.BulkUpdateGroup", "ProxyHostHandler.BulkUpdateSecurityHeaders",
	"ProxyHostHandler.Create", "ProxyHostHandler.TestConnection", "ProxyHostHandler.Update",
	"RedirectionHostHandler.Create", "RedirectionHostHandler.Update",
	"RemoteServerHandler.Create", "RemoteServerHandler.TestConnectionCustom", "RemoteServerHandler.Update",
	"SecurityHeadersHandler.ApplyPreset", "SecurityHeadersHandler.BuildCSP", "SecurityHeadersHandler.CalculateScore",
	"SecurityHeadersHandler.UpdateProfile", "SecurityHeadersHandler.ValidateCSP",
	"SettingsHandler.PatchConfig", "SettingsHandler.SendTestEmail", "SettingsHandler.TestPublicURL",
	"SettingsHandler.UpdateSMTPConfig", "SettingsHandler.UpdateSetting", "SettingsHandler.ValidatePublicURL",
	"UptimeHandler.Create", "UptimeHandler.Update",
	"UserHandler.AcceptInvite", "UserHandler.CreateUser", "UserHandler.InviteUser", "UserHandler.PreviewInviteURL",
	"UserHandler.Setup", "UserHandler.UpdateProfile", "UserHandler.UpdateUser", "UserHandler.UpdateUserPermissions",
}

// bindCallPattern matches a request read in the handlers source. The helpers themselves are defined in files that
// are skipped.
var (
	bindCallPattern   = regexp.MustCompile(`\b(bindJSON|bindQuery|bindSmallJSON)\(`)
	funcHeaderPattern = regexp.MustCompile(`^func \(\w+ \*?(\w+)\) (\w+)\(`)
	bindHelperFiles   = map[string]bool{"request_body.go": true, "request_errors.go": true}
)

// bindSitesInSource returns the "Type.Method" name of every handler method that reads a request, found by scanning the
// handlers source.
func bindSitesInSource(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "handlers", "*.go"))
	require.NoError(t, err)
	sites := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || bindHelperFiles[filepath.Base(file)] {
			continue
		}
		sites = mergeBindSites(sites, scanBindSites(t, file))
	}
	return sites
}

// scanBindSites reads one source file and returns the methods in it that contain a bind call.
func scanBindSites(t *testing.T, file string) map[string]bool {
	t.Helper()
	f, err := os.Open(file)
	require.NoError(t, err)
	defer f.Close()
	sites := map[string]bool{}
	current := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "func ") {
			current = ""
			if m := funcHeaderPattern.FindStringSubmatch(line); m != nil {
				current = m[1] + "." + m[2]
			}
			continue
		}
		if current != "" && bindCallPattern.MatchString(line) {
			sites[current] = true
		}
	}
	require.NoError(t, scanner.Err())
	return sites
}

func mergeBindSites(into, from map[string]bool) map[string]bool {
	for k := range from {
		into[k] = true
	}
	return into
}

// TestBindHandlerTableMatchesTheSource fails when the bind call sites in the handlers source and bindHandlerMethods
// differ, in either direction.
func TestBindHandlerTableMatchesTheSource(t *testing.T) {
	t.Parallel()
	inSource := bindSitesInSource(t)
	inTable := map[string]bool{}
	for _, name := range bindHandlerMethods {
		inTable[name] = true
	}
	var missing, stale []string
	for name := range inSource {
		if !inTable[name] {
			missing = append(missing, name)
		}
	}
	for name := range inTable {
		if !inSource[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	assert.Empty(t, missing, "handler methods that read a request but are not in bindHandlerMethods")
	assert.Empty(t, stale, "bindHandlerMethods entries that no longer read a request")
	assert.NotEmpty(t, inSource, "the source scan found bind call sites")
}

// bindRouteAnswerForbidden lists text a malformed-request answer must never carry: Go names, types, parser text and
// offsets.
var bindRouteAnswerForbidden = []string{
	"Key:", "Error:Field", "struct {", "json:", "models.", "handlers.", "routes.", "cannot unmarshal", "offset",
	"invalid character", "validator", "reflect", "runtime", ".go:",
}

// bindRouteHandlerOf reports which "Type.Method" a route's handler chain ends in, or "" when it is not a bind site.
func bindRouteHandlerOf(handler string, sites map[string]bool) string {
	name := strings.TrimSuffix(handler, "-fm")
	for site := range sites {
		typeName, method, _ := strings.Cut(site, ".")
		if strings.HasSuffix(name, "handlers.(*"+typeName+")."+method) {
			return site
		}
	}
	return ""
}

var pathParamPattern = regexp.MustCompile(`:[A-Za-z]+`)

// setupBindRouteRouter builds the production router (as cmd/api does) with the import routes and an admin bearer token.
func setupBindRouteRouter(t *testing.T) (router *gin.Engine, token string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(routes.IsolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	cfg := config.Config{JWTSecret: "test-secret"}
	router = gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, cfg))
	routes.RegisterImportHandler(router, db, cfg, "", t.TempDir(), "", nil)

	admin := &models.User{Email: "bind-admin@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, db.Create(admin).Error)
	token, err = services.NewAuthService(db, cfg).GenerateToken(admin)
	require.NoError(t, err)
	return router, token
}

// TestRegister_EveryBindSiteAnswersAMalformedRequestPlainly walks the real router. Every route whose handler reads a
// request gets a malformed request as the admin: a JSON array for a body route, a text value for a query route. The
// answer carries no Go detail, and a 400 is the invalid_request answer. Routes with a path parameter are sent a
// placeholder id, so the handler may answer 404 before it reads the request; those are logged, and their bind is
// covered at handler level.
func TestRegister_EveryBindSiteAnswersAMalformedRequestPlainly(t *testing.T) {
	router, token := setupBindRouteRouter(t)
	sites := map[string]bool{}
	for _, name := range bindHandlerMethods {
		sites[name] = true
	}

	reached, own := 0, 0
	for _, route := range router.Routes() {
		site := bindRouteHandlerOf(route.Handler, sites)
		if site == "" {
			continue
		}
		path := pathParamPattern.ReplaceAllString(route.Path, "x")
		var req *http.Request
		if route.Method == http.MethodGet {
			req = httptest.NewRequest(route.Method, path+"?limit=abc&page=abc", http.NoBody)
		} else {
			req = httptest.NewRequest(route.Method, path, strings.NewReader("[1]"))
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()
		for _, word := range bindRouteAnswerForbidden {
			assert.NotContains(t, body, word, "%s %s (%s) answer", route.Method, route.Path, site)
		}
		if w.Code != http.StatusBadRequest {
			// A path-parameter route answers 404 before it reads the request, and a role or state gate can answer 403;
			// the bind of those sites is covered by their handler-level tests.
			t.Logf("%s %s (%s) answered %d before reading the request", route.Method, route.Path, site, w.Code)
			continue
		}
		var answer map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &answer), "%s %s answer is JSON", route.Method, route.Path)
		message, _ := answer["error"].(string)
		assert.NotEmpty(t, message, "%s %s (%s) answer names the problem", route.Method, route.Path, site)
		if !strings.HasPrefix(message, msgInvalidRequestText) {
			// A handler that binds with its own fixed answer (bindSmallJSON with an onInvalid text) keeps that text.
			own++
			continue
		}
		reached++
		assert.Equal(t, "invalid_request", answer["error_code"], "%s %s (%s) answer %s", route.Method, route.Path, site, body)
	}
	assert.GreaterOrEqual(t, reached, 40, "the generic invalid_request answer is reached through the real router")
	t.Logf("generic invalid_request answers: %d; own fixed answers: %d", reached, own)
}

// msgInvalidRequestText is the start of the generic answer every bindJSON or bindQuery failure gets.
const msgInvalidRequestText = "The request is not valid."

// TestRegister_MalformedHostAnswersNameTheFieldAndNothingElse keeps the named-field answers for the host forms: the
// fields the client got wrong are named by their public names only.
func TestRegister_MalformedHostAnswersNameTheFieldAndNothingElse(t *testing.T) {
	router, token := setupBindRouteRouter(t)
	cases := []struct {
		name, method, path, body, check string
	}{
		{"proxy create with a text port", http.MethodPost, "/api/v1/proxy-hosts", `{"forward_port":"abc"}`, "Check: forward_port."},
		{"proxy create with broken JSON", http.MethodPost, "/api/v1/proxy-hosts", `{"name":`, ""},
		{"proxy create with nested brackets", http.MethodPost, "/api/v1/proxy-hosts", "[[[", ""},
		{"bulk acl with a text list", http.MethodPut, "/api/v1/proxy-hosts/bulk-update-acl", `{"host_uuids":"x"}`, "Check: host_uuids."},
		{"redirection create with broken JSON", http.MethodPost, "/api/v1/redirection-hosts", `{"name":`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			var answer map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
			assert.Equal(t, "invalid_request", answer["error_code"])
			if tc.check != "" {
				assert.Contains(t, answer["error"], tc.check)
			}
			for _, word := range bindRouteAnswerForbidden {
				assert.NotContains(t, w.Body.String(), word)
			}
		})
	}
}
