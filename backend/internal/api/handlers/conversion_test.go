package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeIntToUint(t *testing.T) {
	t.Run("ValidPositive", func(t *testing.T) {
		val, ok := safeIntToUint(42)
		assert.True(t, ok)
		assert.Equal(t, uint(42), val)
	})

	t.Run("Zero", func(t *testing.T) {
		val, ok := safeIntToUint(0)
		assert.True(t, ok)
		assert.Equal(t, uint(0), val)
	})

	t.Run("Negative", func(t *testing.T) {
		val, ok := safeIntToUint(-1)
		assert.False(t, ok)
		assert.Equal(t, uint(0), val)
	})
}

func TestSafeFloat64ToUint(t *testing.T) {
	t.Run("ValidPositive", func(t *testing.T) {
		val, ok := safeFloat64ToUint(42.0)
		assert.True(t, ok)
		assert.Equal(t, uint(42), val)
	})

	t.Run("Zero", func(t *testing.T) {
		val, ok := safeFloat64ToUint(0.0)
		assert.True(t, ok)
		assert.Equal(t, uint(0), val)
	})

	t.Run("Negative", func(t *testing.T) {
		val, ok := safeFloat64ToUint(-1.0)
		assert.False(t, ok)
		assert.Equal(t, uint(0), val)
	})

	t.Run("NotInteger", func(t *testing.T) {
		val, ok := safeFloat64ToUint(42.5)
		assert.False(t, ok)
		assert.Equal(t, uint(0), val)
	})
}
