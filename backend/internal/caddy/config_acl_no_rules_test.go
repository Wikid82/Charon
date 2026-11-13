package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

const aclDenyBody = "Access denied: IP not in whitelist"

func TestBuildACLHandler_AllowListWithoutRulesDeniesEveryone(t *testing.T) {
	for name, ipRules := range map[string]string{
		"empty string":    "",
		"empty array":     "[]",
		"unreadable JSON": "invalid-json",
	} {
		t.Run(name, func(t *testing.T) {
			acl := &models.AccessList{Type: "whitelist", IPRules: ipRules, Enabled: true}
			h, err := buildACLHandler(acl, "")
			require.ErrorIs(t, err, errAllowListFailClosed)
			require.NotNil(t, h)

			b, _ := json.Marshal(h)
			require.Contains(t, string(b), aclDenyBody)
			require.NotContains(t, string(b), `"remote_ip"`, "no allowed address means no match clause")
		})
	}
}

func TestBuildACLHandler_AllowListWithoutRulesKeepsAdminWhitelist(t *testing.T) {
	acl := &models.AccessList{Type: "whitelist", IPRules: "", Enabled: true}
	h, err := buildACLHandler(acl, " 10.9.8.7 , 10.9.8.8 ")
	require.ErrorIs(t, err, errAllowListFailClosed)
	require.NotNil(t, h)

	b, _ := json.Marshal(h)
	require.Contains(t, string(b), `"10.9.8.7"`)
	require.Contains(t, string(b), `"10.9.8.8"`)
	require.Contains(t, string(b), aclDenyBody)
}

func TestBuildACLHandler_BlockListWithoutRulesAddsNothing(t *testing.T) {
	for _, ipRules := range []string{"", "[]"} {
		acl := &models.AccessList{Type: "blacklist", IPRules: ipRules, Enabled: true}
		h, err := buildACLHandler(acl, "")
		require.NoError(t, err)
		require.Nil(t, h)
	}
}

func TestGenerateConfig_AllowListWithoutRulesAddsDenyHandlerAndOneWarn(t *testing.T) {
	var buf strings.Builder
	logger.Init(true, &buf)

	acl := models.AccessList{ID: 41, Name: "Empty", Enabled: true, Type: "whitelist", IPRules: ""}
	hostA := models.ProxyHost{UUID: "no-rules-a", DomainNames: "a-norules.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}
	hostB := models.ProxyHost{UUID: "no-rules-b", DomainNames: "b-norules.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{hostA, hostB}, t.TempDir(), "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	routes, err := json.Marshal(cfg.Apps.HTTP.Servers["charon_server"].Routes)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(routes), aclDenyBody), "each host gets a deny handler")

	require.Equal(t, 1, strings.Count(buf.String(), "Access list is enabled but has no usable IP rules"), "one Warn per apply")
}

func TestGenerateConfig_BlockListWithoutRulesLogsNothing(t *testing.T) {
	var buf strings.Builder
	logger.Init(true, &buf)

	acl := models.AccessList{ID: 42, Name: "Empty block", Enabled: true, Type: "blacklist", IPRules: ""}
	host := models.ProxyHost{UUID: "block-no-rules", DomainNames: "block-norules.example.com", Enabled: true, ForwardHost: "app", ForwardPort: 8080, AccessListID: &acl.ID, AccessList: &acl}

	cfg, err := GenerateConfig([]models.ProxyHost{host}, t.TempDir(), "", "", "", false, false, false, false, true, "", nil, nil, nil, nil, nil)
	require.NoError(t, err)

	routes, err := json.Marshal(cfg.Apps.HTTP.Servers["charon_server"].Routes)
	require.NoError(t, err)
	require.NotContains(t, string(routes), aclDenyBody)
	require.NotContains(t, buf.String(), "no usable IP rules")
}
