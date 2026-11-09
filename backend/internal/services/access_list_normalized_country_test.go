package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestAccessListService_Create_ValidatesNormalizedCountryCodes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "surrounding spaces", input: " se , ", want: "SE"},
		{name: "trailing comma", input: "se,us,", want: "SE,US"},
		{name: "lower case", input: "ca", want: "CA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewAccessListService(setupTestDB(t))
			acl := &models.AccessList{Name: "Geo", Type: "geo_whitelist", CountryCodes: tc.input, Enabled: true}

			require.NoError(t, service.Create(acl))
			assert.Equal(t, tc.want, acl.CountryCodes)
		})
	}
}

func TestAccessListService_Create_BlankCountryCodesAreRequired(t *testing.T) {
	service := NewAccessListService(setupTestDB(t))
	acl := &models.AccessList{Name: "Geo", Type: "geo_blacklist", CountryCodes: " , ", Enabled: true}

	err := service.Create(acl)
	require.ErrorContains(t, err, "country codes are required")
}

func TestAccessListService_Create_RejectsInvalidNormalizedCountryCode(t *testing.T) {
	service := NewAccessListService(setupTestDB(t))
	acl := &models.AccessList{Name: "Geo", Type: "geo_blacklist", CountryCodes: " zz , ", Enabled: true}

	err := service.Create(acl)
	require.ErrorIs(t, err, ErrInvalidCountryCode)
	require.ErrorContains(t, err, "invalid country code: ZZ")
}

func TestAccessListService_Update_ValidatesNormalizedCountryCodes(t *testing.T) {
	service := NewAccessListService(setupTestDB(t))
	acl := &models.AccessList{Name: "Geo", Type: "geo_whitelist", CountryCodes: "US", Enabled: true}
	require.NoError(t, service.Create(acl))

	err := service.Update(acl.ID, &models.AccessList{Name: "Geo", Type: "geo_whitelist", CountryCodes: " ca , ", Enabled: true})
	require.NoError(t, err)

	stored, err := service.GetByID(acl.ID)
	require.NoError(t, err)
	assert.Equal(t, "CA", stored.CountryCodes)
}
