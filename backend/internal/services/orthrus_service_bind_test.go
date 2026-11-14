package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/services"
)

func TestOrthrusService_Patch_ExternalProxyBind(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "automatic", in: "", want: ""},
		{name: "blank is automatic", in: "   ", want: ""},
		{name: "all IPv4", in: "0.0.0.0", want: "0.0.0.0"},
		{name: "all IPv6", in: "::", want: "::"},
		{name: "address", in: " 172.18.0.4 ", want: "172.18.0.4"},
		{name: "subnet stored masked", in: "172.18.0.7/16", want: "172.18.0.0/16"},
		{name: "zone rejected", in: "fe80::1%eth0", wantErr: true},
		{name: "hostname rejected", in: "localhost", wantErr: true},
		{name: "too long", in: strings.Repeat("1", 65), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, uuid := newBindTestService(t)
			bind := tc.in
			agent, err := svc.Patch(uuid, services.AgentPatch{ExternalProxyBind: &bind})
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, services.ErrInvalidExternalProxyBind))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, agent.ExternalProxyBind)
		})
	}
}

func TestOrthrusService_Patch_ExternalProxyBindOmittedKeepsValue(t *testing.T) {
	svc, uuid := newBindTestService(t)
	subnet := "10.0.0.0/8"
	_, err := svc.Patch(uuid, services.AgentPatch{ExternalProxyBind: &subnet})
	require.NoError(t, err)

	port := 23750
	agent, err := svc.Patch(uuid, services.AgentPatch{ExternalProxyPort: &port})
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8", agent.ExternalProxyBind)
}

func newBindTestService(t *testing.T) (svc *services.OrthrusService, uuid string) {
	t.Helper()
	db := setupOrthrusTestDB(t)
	srv := setupOrthrusServer(t, db)
	t.Cleanup(srv.Stop)

	svc = services.NewOrthrusService(db, srv)
	agent, _, err := svc.Provision("bind-agent")
	require.NoError(t, err)
	return svc, agent.UUID
}
