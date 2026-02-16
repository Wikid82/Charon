package handlers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsProviderValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "invalid custom template", err: errors.New("invalid custom template: parse failed"), want: true},
		{name: "rendered template", err: errors.New("rendered template invalid JSON"), want: true},
		{name: "failed parse", err: errors.New("failed to parse template"), want: true},
		{name: "failed render", err: errors.New("failed to render template"), want: true},
		{name: "invalid discord url", err: errors.New("invalid Discord webhook URL"), want: true},
		{name: "other", err: errors.New("database unavailable"), want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, isProviderValidationError(testCase.err))
		})
	}
}
