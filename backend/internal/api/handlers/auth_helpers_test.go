package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequireUserID_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	userID, ok := requireUserID(ctx)

	assert.False(t, ok)
	assert.Equal(t, uint(0), userID)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireUserID_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	ctx.Set("userID", "not-a-uint")

	userID, ok := requireUserID(ctx)

	assert.False(t, ok)
	assert.Equal(t, uint(0), userID)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireUserID_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	ctx.Set("userID", uint(42))

	userID, ok := requireUserID(ctx)

	assert.True(t, ok)
	assert.Equal(t, uint(42), userID)
}
