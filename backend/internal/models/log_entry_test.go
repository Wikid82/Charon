package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFilterValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      LogFilter
		want    LogFilter
		wantErr string
	}{
		{
			name: "defaults applied for zero values",
			in:   LogFilter{},
			want: LogFilter{Limit: 50, Offset: 0, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "valid values preserved",
			in:   LogFilter{Limit: 100, Offset: 10, Sort: "asc", SortBy: "level"},
			want: LogFilter{Limit: 100, Offset: 10, Sort: "asc", SortBy: "level"},
		},
		{
			name: "limit zero treated as unset default 50",
			in:   LogFilter{Limit: 0, SortBy: "ts"},
			want: LogFilter{Limit: 50, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "negative limit treated as unset default 50",
			in:   LogFilter{Limit: -5},
			want: LogFilter{Limit: 50, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "limit clamped to 500",
			in:   LogFilter{Limit: 9999},
			want: LogFilter{Limit: 500, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "negative offset clamped to zero",
			in:   LogFilter{Offset: -10},
			want: LogFilter{Limit: 50, Offset: 0, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "sort direction lowercased",
			in:   LogFilter{Sort: "ASC"},
			want: LogFilter{Limit: 50, Sort: "asc", SortBy: "ts"},
		},
		{
			name: "invalid sort direction falls back to desc",
			in:   LogFilter{Sort: "sideways"},
			want: LogFilter{Limit: 50, Sort: "desc", SortBy: "ts"},
		},
		{
			name: "sort_by lowercased",
			in:   LogFilter{SortBy: "STATUS"},
			want: LogFilter{Limit: 50, Sort: "desc", SortBy: "status"},
		},
		{
			name:    "invalid sort_by rejected",
			in:      LogFilter{SortBy: "password"},
			wantErr: "invalid sort_by: must be one of ts, level, method, uri, status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := tt.in
			err := f.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.Limit, f.Limit)
			assert.Equal(t, tt.want.Offset, f.Offset)
			assert.Equal(t, tt.want.Sort, f.Sort)
			assert.Equal(t, tt.want.SortBy, f.SortBy)
		})
	}
}

func TestLogFilterValidate_AllAllowlistedSortFields(t *testing.T) {
	t.Parallel()

	for field := range ValidSortFields {
		f := LogFilter{SortBy: field}
		require.NoError(t, f.Validate(), "sort_by %q must be accepted", field)
		assert.Equal(t, field, f.SortBy)
	}
	// The allowlist is exactly the five documented fields.
	assert.Len(t, ValidSortFields, 5)
	for _, field := range []string{"ts", "level", "method", "uri", "status"} {
		assert.True(t, ValidSortFields[field], "expected %q in allowlist", field)
	}
}
