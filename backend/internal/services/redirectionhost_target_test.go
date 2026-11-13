package services

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestRedirectTargetRejectsReservedCharacters(t *testing.T) {
	svc := &RedirectionHostService{}
	tests := []struct {
		name      string
		targetURL string
		wantErr   bool
	}{
		{name: "plain target accepted", targetURL: "https://new.example.com/path?q=1", wantErr: false},
		{name: "percent-encoded braces accepted", targetURL: "https://new.example.com/%7Benv.X%7D", wantErr: false},
		{name: "literal open brace refused", targetURL: "https://new.example.com/{env.X}", wantErr: true},
		{name: "literal close brace refused", targetURL: "https://new.example.com/a}", wantErr: true},
		{name: "brace in query refused", targetURL: "https://new.example.com/?k={http.request.uri}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &models.RedirectionHost{
				DomainNames: "old.example.com",
				TargetURL:   tt.targetURL,
				StatusCode:  301,
			}
			err := svc.validateRedirectionHost(host)
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrRedirectTargetInvalid)
			// The fixed message must not echo the submitted value.
			assert.NotContains(t, err.Error(), tt.targetURL)
		})
	}
}

func TestWarnRefusedRedirectTargets_WarnsOncePerRowNamingHostNotValue(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	rows := []models.RedirectionHost{
		{UUID: "rh-braced", DomainNames: "braced.example.com", TargetURL: "https://new.example.com/{env.SECRET_VALUE}", StatusCode: 301, Enabled: true},
		{UUID: "rh-clean", DomainNames: "clean.example.com", TargetURL: "https://new.example.com/%7Benv.X%7D", StatusCode: 301, Enabled: true},
	}
	require.NoError(t, db.Create(&rows).Error)

	var buf bytes.Buffer
	logger.Init(false, &buf)
	t.Cleanup(func() { logger.Init(false, &bytes.Buffer{}) })

	refused := WarnRefusedRedirectTargets(db)
	assert.Equal(t, 1, refused)

	out := buf.String()
	assert.Equal(t, 1, strings.Count(out, "braced.example.com"), "exactly one warning for the refused row")
	assert.NotContains(t, out, "clean.example.com")
	assert.NotContains(t, out, "SECRET_VALUE", "the warning must not echo the stored target")
}

func TestWarnRefusedRedirectTargets_DBErrorReturnsZero(t *testing.T) {
	db := setupRedirectionHostTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Equal(t, 0, WarnRefusedRedirectTargets(db))
}
