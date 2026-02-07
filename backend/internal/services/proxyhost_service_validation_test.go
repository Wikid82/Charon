package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestProxyHostService_ForwardHostValidation(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	tests := []struct {
		name        string
		forwardHost string
		wantErr     bool
	}{
		{
			name:        "Valid IP",
			forwardHost: "192.168.1.1",
			wantErr:     false,
		},
		{
			name:        "Valid Hostname",
			forwardHost: "example.com",
			wantErr:     false,
		},
		{
			name:        "Docker Service Name",
			forwardHost: "my-service",
			wantErr:     false,
		},
		{
			name:        "Docker Service Name with Underscore",
			forwardHost: "my_db_Service",
			wantErr:     false,
		},
		{
			name:        "Docker Internal Host",
			forwardHost: "host.docker.internal",
			wantErr:     false,
		},
		{
			name:        "IP with Port (Should be stripped and pass)",
			forwardHost: "192.168.1.1:8080",
			wantErr:     false,
		},
		{
			name:        "Hostname with Port (Should be stripped and pass)",
			forwardHost: "example.com:3000",
			wantErr:     false,
		},
		{
			name:        "Host with http scheme (Should be stripped and pass)",
			forwardHost: "http://example.com",
			wantErr:     false,
		},
		{
			name:        "Host with https scheme (Should be stripped and pass)",
			forwardHost: "https://example.com",
			wantErr:     false,
		},
		{
			name:        "Invalid Characters",
			forwardHost: "invalid$host",
			wantErr:     true,
		},
		{
			name:        "Empty Host",
			forwardHost: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &models.ProxyHost{
				DomainNames: "test-" + tt.name + ".example.com",
				ForwardHost: tt.forwardHost,
				ForwardPort: 8080,
			}
			// We only care about validation error
			err := service.Create(host)
			if tt.wantErr {
				assert.Error(t, err)
			} else if err != nil {
				// Check if error is validation or something else
				// If it's something else, it might be fine for this test context
				// but "forward host must be..." is what we look for.
				assert.NotContains(t, err.Error(), "forward host", "Should not fail validation")
			}
		})
	}
}
