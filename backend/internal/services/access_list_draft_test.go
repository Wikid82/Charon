package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestValidateDraft_AcceptsDraftWithoutName(t *testing.T) {
	svc := NewAccessListService(nil)

	err := svc.ValidateDraft(&models.AccessList{Type: "geo_whitelist", CountryCodes: "DE", Enabled: true})

	require.NoError(t, err)
}

func TestValidateDraft_RefusesSameRulesAsCreate(t *testing.T) {
	tests := []struct {
		name string
		acl  models.AccessList
		want error
	}{
		{name: "invalid type", acl: models.AccessList{Type: "bogus"}, want: ErrInvalidAccessListType},
		{name: "malformed rules", acl: models.AccessList{Type: "whitelist", IPRules: "{"}},
		{name: "bad cidr", acl: models.AccessList{Type: "whitelist", IPRules: `[{"cidr":"nope"}]`}, want: ErrInvalidIPAddress},
		{name: "enabled allow-list without rules", acl: models.AccessList{Type: "whitelist", Enabled: true}, want: ErrAccessListRulesRequired},
		{name: "geo list without codes", acl: models.AccessList{Type: "geo_blacklist"}},
		{name: "geo list with bad code", acl: models.AccessList{Type: "geo_blacklist", CountryCodes: "ZZ"}, want: ErrInvalidCountryCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewAccessListService(nil)

			err := svc.ValidateDraft(&tt.acl)

			require.Error(t, err)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
			}
		})
	}
}

func TestValidateAccessList_StillRequiresName(t *testing.T) {
	svc := NewAccessListService(nil)

	err := svc.validateAccessList(&models.AccessList{Type: "blacklist"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}
