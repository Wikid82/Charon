package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestSecurityService_UpsertRuleSet_RefusesContentWithoutRules(t *testing.T) {
	cases := map[string]string{
		"blank":                        "",
		"whitespace":                   "  \n\t\n",
		"comment only":                 "# SecRule ARGS x id:1,deny",
		"engine directives":            "SecRuleEngine On\nSecRequestBodyAccess On\n",
		"source url in place of rules": "https://example.com/owasp-crs.tar.gz",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			db := setupSecurityTestDB(t)
			svc := newTestSecurityService(t, db)

			err := svc.UpsertRuleSet(&models.SecurityRuleSet{Name: "owasp-crs", Content: content})
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrRuleSetEmpty))

			list, err := svc.ListRuleSets()
			require.NoError(t, err)
			assert.Empty(t, list, "a refused rule set must not be stored")
		})
	}
}

func TestSecurityService_UpsertRuleSet_UpdateBlankingContentIsRefused(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	rs := &models.SecurityRuleSet{Name: "owasp-crs", Content: testRuleSetContent}
	require.NoError(t, svc.UpsertRuleSet(rs))

	err := svc.UpsertRuleSet(&models.SecurityRuleSet{Name: "owasp-crs", Content: "# cleared"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRuleSetEmpty))

	list, err := svc.ListRuleSets()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, testRuleSetContent, list[0].Content, "the stored content must not be overwritten")
}

func TestSecurityService_UpsertRuleSet_NameCheckedBeforeRules(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	err := svc.UpsertRuleSet(&models.SecurityRuleSet{Content: ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name required")
	assert.False(t, errors.Is(err, ErrRuleSetEmpty))
}

func TestSecurityService_UpsertRuleSet_SizeCheckedBeforeRules(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	err := svc.UpsertRuleSet(&models.SecurityRuleSet{Name: "big", Content: strings.Repeat("#", 2*1024*1024+1)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
	assert.False(t, errors.Is(err, ErrRuleSetEmpty))
}

func TestSecurityService_UpsertRuleSet_NilPayloadIsNoOp(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	assert.NoError(t, svc.UpsertRuleSet(nil))

	list, err := svc.ListRuleSets()
	require.NoError(t, err)
	assert.Empty(t, list)
}
