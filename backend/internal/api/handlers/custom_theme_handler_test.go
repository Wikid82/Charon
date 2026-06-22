package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupCustomThemeHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:custom_theme_handler_test_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CustomTheme{}))
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

// buildThemeRouter creates a test router that simulates the auth middleware.
// If authenticated is false, all requests return 401.
func buildThemeRouter(db *gorm.DB, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if !authenticated {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}
		c.Set("userID", uint(1))
		c.Set("role", "user")
		c.Next()
	})
	h := NewCustomThemeHandler(db)
	r.GET("/themes", h.ListThemes)
	r.POST("/themes", h.CreateTheme)
	r.PUT("/themes/:id", h.UpdateTheme)
	r.DELETE("/themes/:id", h.DeleteTheme)
	return r
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// UT-01: GET returns empty array [] (not null) when no themes exist.
func TestCustomThemeHandler_ListThemes_EmptyArray(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	req, _ := http.NewRequest(http.MethodGet, "/themes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String())
}

// UT-02: POST creates with non-empty UUID id and returns 201.
func TestCustomThemeHandler_CreateTheme_Success(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	body := jsonBody(t, map[string]string{
		"name":   "My Dark Theme",
		"colors": `{"bgBase":"15 23 42"}`,
	})
	req, _ := http.NewRequest(http.MethodPost, "/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp models.CustomTheme
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "My Dark Theme", resp.Name)
	assert.Len(t, resp.ID, 36, "ID should be a UUID")
}

// UT-03: POST duplicate name returns 409.
func TestCustomThemeHandler_CreateTheme_DuplicateName(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	body1 := jsonBody(t, map[string]string{"name": "Duplicate", "colors": `{"bgBase":"0 0 0"}`})
	req1, _ := http.NewRequest(http.MethodPost, "/themes", body1)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	body2 := jsonBody(t, map[string]string{"name": "Duplicate", "colors": `{"bgBase":"255 255 255"}`})
	req2, _ := http.NewRequest(http.MethodPost, "/themes", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), "a theme with that name already exists")
}

// UT-04: POST empty name returns 400.
func TestCustomThemeHandler_CreateTheme_EmptyName(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	body := jsonBody(t, map[string]string{"name": "", "colors": `{"bgBase":"0 0 0"}`})
	req, _ := http.NewRequest(http.MethodPost, "/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// UT-05: POST invalid JSON in colors returns 400.
func TestCustomThemeHandler_CreateTheme_InvalidColorsJSON(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	body := jsonBody(t, map[string]string{"name": "Bad Colors", "colors": `{invalid json}`})
	req, _ := http.NewRequest(http.MethodPost, "/themes", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "colors must be valid JSON")
}

// UT-06: PUT updates name and/or colors, returns 200 with updated record.
func TestCustomThemeHandler_UpdateTheme_Success(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	// Create first
	createBody := jsonBody(t, map[string]string{"name": "Original", "colors": `{"bgBase":"0 0 0"}`})
	createReq, _ := http.NewRequest(http.MethodPost, "/themes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created models.CustomTheme
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Update name
	newName := "Updated Name"
	updateBody := jsonBody(t, map[string]any{"name": newName})
	updateReq, _ := http.NewRequest(http.MethodPut, "/themes/"+created.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	assert.Equal(t, http.StatusOK, updateW.Code)

	var updated models.CustomTheme
	require.NoError(t, json.Unmarshal(updateW.Body.Bytes(), &updated))
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, `{"bgBase":"0 0 0"}`, updated.Colors)
}

// UT-06b: PUT updates colors only.
func TestCustomThemeHandler_UpdateTheme_ColorsOnly(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	createBody := jsonBody(t, map[string]string{"name": "Color Test", "colors": `{"bgBase":"0 0 0"}`})
	createReq, _ := http.NewRequest(http.MethodPost, "/themes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created models.CustomTheme
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	newColors := `{"bgBase":"255 255 255"}`
	updateBody := jsonBody(t, map[string]any{"colors": newColors})
	updateReq, _ := http.NewRequest(http.MethodPut, "/themes/"+created.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	assert.Equal(t, http.StatusOK, updateW.Code)

	var updated models.CustomTheme
	require.NoError(t, json.Unmarshal(updateW.Body.Bytes(), &updated))
	assert.Equal(t, "Color Test", updated.Name)
	assert.Equal(t, newColors, updated.Colors)
}

// UT-07: PUT nonexistent id returns 404.
func TestCustomThemeHandler_UpdateTheme_NotFound(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	newName := "Ghost"
	body := jsonBody(t, map[string]any{"name": &newName})
	req, _ := http.NewRequest(http.MethodPut, "/themes/nonexistent-uuid", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// UT-06c: PUT with empty name returns 400.
func TestCustomThemeHandler_UpdateTheme_EmptyName(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	createBody := jsonBody(t, map[string]string{"name": "Some Theme", "colors": `{"bgBase":"0 0 0"}`})
	createReq, _ := http.NewRequest(http.MethodPost, "/themes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created models.CustomTheme
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Empty name in PUT
	body := jsonBody(t, map[string]string{"name": ""})
	req, _ := http.NewRequest(http.MethodPut, "/themes/"+created.ID, body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "name cannot be empty")
}

// UT-08: DELETE removes record and returns 200 {"message":"theme deleted"}.
func TestCustomThemeHandler_DeleteTheme_Success(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	// Create first
	createBody := jsonBody(t, map[string]string{"name": "To Delete", "colors": `{"bgBase":"0 0 0"}`})
	createReq, _ := http.NewRequest(http.MethodPost, "/themes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created models.CustomTheme
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	// Delete
	delReq, _ := http.NewRequest(http.MethodDelete, "/themes/"+created.ID, nil)
	delW := httptest.NewRecorder()
	r.ServeHTTP(delW, delReq)

	assert.Equal(t, http.StatusOK, delW.Code)
	assert.Contains(t, delW.Body.String(), "theme deleted")

	// Verify deleted from DB
	var theme models.CustomTheme
	assert.Error(t, db.First(&theme, "id = ?", created.ID).Error)
}

// UT-09: DELETE nonexistent returns 404.
func TestCustomThemeHandler_DeleteTheme_NotFound(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	req, _ := http.NewRequest(http.MethodDelete, "/themes/nonexistent-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// UT-10: Unauthenticated calls return 401.
func TestCustomThemeHandler_Unauthenticated(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, false)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/themes"},
		{http.MethodPost, "/themes"},
		{http.MethodPut, "/themes/some-id"},
		{http.MethodDelete, "/themes/some-id"},
	}

	for _, tt := range tests {
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 for %s %s", tt.method, tt.path)
	}
}

// Verify ListThemes returns populated array after create.
func TestCustomThemeHandler_ListThemes_AfterCreate(t *testing.T) {
	db := setupCustomThemeHandlerDB(t)
	r := buildThemeRouter(db, true)

	createBody := jsonBody(t, map[string]string{"name": "Listed Theme", "colors": `{"bgBase":"0 0 0"}`})
	createReq, _ := http.NewRequest(http.MethodPost, "/themes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	listReq, _ := http.NewRequest(http.MethodGet, "/themes", nil)
	listW := httptest.NewRecorder()
	r.ServeHTTP(listW, listReq)

	assert.Equal(t, http.StatusOK, listW.Code)

	var themes []models.CustomTheme
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &themes))
	assert.Len(t, themes, 1)
	assert.Equal(t, "Listed Theme", themes[0].Name)
}
