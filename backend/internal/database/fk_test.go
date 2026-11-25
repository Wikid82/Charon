package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type codedError struct{ code int }

func (e codedError) Error() string { return "sqlite error" }
func (e codedError) Code() int     { return e.code }

func TestIsForeignKeyViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "driver code 787", err: codedError{code: 787}, want: true},
		{name: "wrapped driver code", err: fmt.Errorf("delete: %w", codedError{code: 787}), want: true},
		{name: "other driver code", err: codedError{code: 19}, want: false},
		{name: "cgo message", err: errors.New("FOREIGN KEY constraint failed"), want: true},
		{name: "unrelated error", err: errors.New("disk full"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsForeignKeyViolation(tt.err))
		})
	}
}
