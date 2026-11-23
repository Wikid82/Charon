package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probeFile struct {
	Content string `json:"content" binding:"required"`
}

type probeRequest struct {
	Password string      `json:"password" binding:"required,min=8"`
	Email    string      `json:"email" binding:"required,email"`
	Count    int         `json:"count"`
	Secret   string      `binding:"required"`
	Files    []probeFile `json:"files" binding:"dive"`
}

type probeBase struct {
	Name string `json:"name" binding:"required"`
}

type probeEmbedded struct {
	probeBase
	Title string `json:"title" binding:"required"`
}

type probeQuery struct {
	Limit int `form:"limit" binding:"min=1"`
}

// probeContext returns a gin context for a request with body, carrying a debug logger that writes to logs.
func probeContext(t *testing.T, method, target, body string, logs *strings.Builder) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	log.SetOutput(logs)
	c.Set("logger", logrus.NewEntry(log))
	return c, w
}

func TestBindFailureAnswersWithThePlainText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		body   string
		fields []string
	}{
		{"syntax", `{"password":`, nil},
		{"wrong type names the field", `{"count":"abc"}`, []string{"count"}},
		{"missing required fields are listed sorted", `{"password":"x"}`, []string{"email", "password"}},
		{"nested path reports the top-level name", `{"password":"longenough","email":"a@b.co","files":[{"content":""}]}`, []string{"files"}},
		{"untagged field is never named", `{"password":"longenough","email":"a@b.co","files":[]}`, nil},
		{"empty body", ``, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs strings.Builder
			c, w := probeContext(t, http.MethodPost, "/x", tc.body, &logs)
			var payload probeRequest

			ok := bindJSON(c, &payload)

			require.False(t, ok)
			require.Equal(t, http.StatusBadRequest, w.Code)
			answer := decodeBody(t, w)
			assert.Equal(t, invalidRequestText(tc.fields), answer["error"])
			assert.Equal(t, codeInvalidRequest, answer["error_code"])
			if tc.fields == nil {
				assert.NotContains(t, answer, "fields")
			} else {
				assert.Len(t, answer["fields"], len(tc.fields))
			}
		})
	}
}

func TestBindFailureNeverEchoesGoDetails(t *testing.T) {
	t.Parallel()
	bodies := []string{`{"password":`, `{"count":"abc"}`, `{"password":"x"}`, `{"files":[{"content":""}]}`, `{"name":1}`}
	forbidden := []string{"Key:", "Error:Field", "struct", "json:", "probeRequest", "probeBase", "validator", "offset", "invalid character", "cannot unmarshal", "int", "string"}
	for _, body := range bodies {
		var logs strings.Builder
		c, w := probeContext(t, http.MethodPost, "/x", body, &logs)
		var payload probeRequest
		bindJSON(c, &payload)
		for _, word := range forbidden {
			assert.NotContains(t, w.Body.String(), word, "body %q", body)
		}
	}
}

func TestPlainTextWithAndWithoutFields(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "The request is not valid.", invalidRequestText(nil))
	assert.Equal(t, "The request is not valid. Check: password.", invalidRequestText([]string{"password"}))
	assert.Equal(t, "The request is not valid. Check: email, password.", invalidRequestText([]string{"email", "password"}))
}

func TestPublicFieldNamesComeFromTheTags(t *testing.T) {
	t.Parallel()
	names := tagNamesOf(reflect.TypeOf(probeEmbedded{}))
	assert.True(t, names["name"], "an embedded struct's tag counts")
	assert.True(t, names["title"])
	assert.False(t, names["Title"], "a Go field name is not a public name")
	assert.Empty(t, invalidFields(nil, probeRequest{}), "no error yields no fields")
}

func TestBindQueryNamesTheFormField(t *testing.T) {
	t.Parallel()
	var logs strings.Builder
	c, w := probeContext(t, http.MethodGet, "/x?limit=0", "", &logs)
	var q probeQuery
	require.False(t, bindQuery(c, &q))
	assert.Equal(t, "The request is not valid. Check: limit.", decodeBody(t, w)["error"])
}

// The debug log holds the error class and the field names, never the request content or the error text.
func TestBindFailureLogsOnlyTheClassAndFieldNames(t *testing.T) {
	t.Parallel()
	var logs strings.Builder
	c, _ := probeContext(t, http.MethodPost, "/x", `{"password":"secret-value-123","count":"abc"}`, &logs)
	var payload probeRequest
	require.False(t, bindJSON(c, &payload))

	out := logs.String()
	assert.Contains(t, out, "count")
	assert.Contains(t, out, "type")
	assert.NotContains(t, out, "secret-value-123", "request content is not logged")
	assert.NotContains(t, out, "cannot unmarshal", "the decoder text is not logged")
}

// An oversized body is answered with 413, not with the validation text.
func TestOversizedBodyIsAnsweredWith413(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", limitRequestBody(16), func(c *gin.Context) {
		var payload probeRequest
		if bindJSON(c, &payload) {
			c.Status(http.StatusOK)
		}
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(`{"password":"`+strings.Repeat("a", 64)+`"}`)))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, codeRequestTooLarge, decodeBody(t, w)["error_code"])
}

// Host bodies bound into a map name no field, whatever is wrong with them.
func TestMapBodiesNameNoField(t *testing.T) {
	t.Parallel()
	var logs strings.Builder
	c, w := probeContext(t, http.MethodPost, "/x", `[1]`, &logs)
	var payload map[string]any
	require.False(t, bindJSON(c, &payload))
	assert.NotContains(t, decodeBody(t, w), "fields")
}
