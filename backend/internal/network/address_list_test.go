package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAddressList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty list", input: "", want: nil},
		{name: "only separators and blanks", input: " , ,", want: nil},
		{name: "single address", input: "192.0.2.1", want: []string{"192.0.2.1"}},
		{
			name:  "trims spaces and drops blank entries",
			input: " 10.0.0.0/8 ,, 192.0.2.1 ,",
			want:  []string{"10.0.0.0/8", "192.0.2.1"},
		},
		{name: "keeps IPv6 entries", input: "2001:db8::1,fd00::/8", want: []string{"2001:db8::1", "fd00::/8"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SplitAddressList(tt.input))
		})
	}
}
