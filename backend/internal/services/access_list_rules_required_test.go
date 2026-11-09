package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAccessListService_RulesRequiredOnSave(t *testing.T) {
	const oneRule = `[{"cidr":"192.168.1.0/24"}]`

	tests := []struct {
		name    string
		acl     models.AccessList
		wantErr error
	}{
		{
			name:    "enabled allow-list without rules is refused",
			acl:     models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, IPRules: ""},
			wantErr: ErrAccessListRulesRequired,
		},
		{
			name:    "enabled allow-list with empty rule array is refused",
			acl:     models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, IPRules: "[]"},
			wantErr: ErrAccessListRulesRequired,
		},
		{
			name: "enabled allow-list with local network option needs no rules",
			acl:  models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, LocalNetworkOnly: true},
		},
		{
			name: "enabled allow-list with rules is accepted",
			acl:  models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, IPRules: oneRule},
		},
		{
			name: "disabled allow-list without rules is accepted",
			acl:  models.AccessList{Name: "wl", Type: "whitelist", Enabled: false, IPRules: ""},
		},
		{
			name: "enabled block-list without rules is accepted",
			acl:  models.AccessList{Name: "bl", Type: "blacklist", Enabled: true, IPRules: ""},
		},
		{
			name: "disabled block-list without rules is accepted",
			acl:  models.AccessList{Name: "bl", Type: "blacklist", Enabled: false, IPRules: "[]"},
		},
	}

	for _, tt := range tests {
		t.Run("create: "+tt.name, func(t *testing.T) {
			service := NewAccessListService(setupTestDB(t))
			acl := tt.acl
			err := service.Create(&acl)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("update: enabling an allow-list without rules is refused", func(t *testing.T) {
		service := NewAccessListService(setupTestDB(t))
		acl := models.AccessList{Name: "wl", Type: "whitelist", Enabled: false, IPRules: oneRule}
		require.NoError(t, service.Create(&acl))

		err := service.Update(acl.ID, &models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, IPRules: ""})
		require.ErrorIs(t, err, ErrAccessListRulesRequired)
	})

	t.Run("update: a disabled allow-list keeps saving without rules", func(t *testing.T) {
		service := NewAccessListService(setupTestDB(t))
		acl := models.AccessList{Name: "wl", Type: "whitelist", Enabled: true, IPRules: oneRule}
		require.NoError(t, service.Create(&acl))

		require.NoError(t, service.Update(acl.ID, &models.AccessList{Name: "wl", Type: "whitelist", Enabled: false, IPRules: ""}))
	})
}
