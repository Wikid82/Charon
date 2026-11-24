package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/util"
)

const (
	codeInvalidRequest = "invalid_request"
	msgInvalidRequest  = "The request is not valid."
	// fieldNameHidden is the tag-name result for a field with neither a json nor a form name. It is never reported.
	fieldNameHidden = "-"
)

// init registers the public field-name function on gin's validator, before any route exists. Validation errors
// then name fields by their json or form tag, never by their Go name.
func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(publicFieldName)
	}
}

// publicFieldName is the tag-name function: the json name, else the form name, else fieldNameHidden.
func publicFieldName(fld reflect.StructField) string {
	if name := tagFieldName(fld.Tag.Get("json")); name != "" {
		return name
	}
	if name := tagFieldName(fld.Tag.Get("form")); name != "" {
		return name
	}
	return fieldNameHidden
}

// tagFieldName returns the name part of a json or form tag, or "" for an absent or skipped name.
func tagFieldName(tag string) string {
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return ""
	}
	return name
}

// bindJSON binds the request body into obj. On failure it answers and returns false: 413 for a body over its limit,
// otherwise 400 invalid_request.
func bindJSON(c *gin.Context, obj any) bool {
	return bindWith(c, obj, c.ShouldBindJSON)
}

// bindQuery binds the query string into obj, answering failures as bindJSON does.
func bindQuery(c *gin.Context, obj any) bool {
	return bindWith(c, obj, c.ShouldBindQuery)
}

func bindWith(c *gin.Context, obj any, bind func(any) error) bool {
	err := bind(obj)
	if err == nil {
		return true
	}
	if respondIfBodyTooLarge(c, err) {
		return false
	}
	respondInvalidRequest(c, err, obj)
	return false
}

// respondInvalidRequest answers 400 with the fixed text, the code and the public names of the fields that failed
// It logs the error class and the field names at debug level and never the error itself or the request content.
// obj is the bound value (or the decode target) whose tag set limits the reported names.
func respondInvalidRequest(c *gin.Context, err error, obj any) {
	fields := invalidFields(err, obj)
	middleware.GetRequestLogger(c).
		WithField("error_class", errorClassOf(err)).
		WithField("fields", fields).
		Debug("Request validation failed")
	body := gin.H{"error": invalidRequestText(fields), "error_code": codeInvalidRequest}
	if len(fields) > 0 {
		body["fields"] = fields
	}
	c.JSON(http.StatusBadRequest, body)
}

// invalidRequestText is the plain answer; it names the fields to fix when there are any.
func invalidRequestText(fields []string) string {
	if len(fields) == 0 {
		return msgInvalidRequest
	}
	return msgInvalidRequest + " Check: " + strings.Join(fields, ", ") + "."
}

// errorClassOf names the kind of a bind or decode failure for the debug log.
func errorClassOf(err error) string {
	var ve validator.ValidationErrors
	var ute *json.UnmarshalTypeError
	var syntax *json.SyntaxError
	switch {
	case errors.As(err, &ve):
		return "validation"
	case errors.As(err, &ute):
		return "type"
	case errors.As(err, &syntax):
		return "syntax"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "empty"
	default:
		return "other"
	}
}

// invalidFields returns the public names from err that belong to obj's own json or form tags, sorted and without
// duplicates. A nested path (files[0].content) is reported by its top-level name. Any name that is not a tag of obj's
// type is dropped, so no Go field name can reach the client.
func invalidFields(err error, obj any) []string {
	allowed := tagNamesOf(reflect.TypeOf(obj))
	var candidates []string
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			candidates = append(candidates, validationFieldName(fe.Namespace()))
		}
	}
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		candidates = append(candidates, topLevelName(ute.Field))
	}

	seen := make(map[string]bool, len(candidates))
	fields := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if !allowed[name] || seen[name] {
			continue
		}
		seen[name] = true
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

// validationFieldName reduces a validator namespace to the top-level public name. The namespace starts with the struct
// type name, which is dropped first. An embedded struct carries no name of its own and shows as fieldNameHidden in the
// path (zzReq.-.password), so those elements are skipped and the field inside the embedded struct is named.
func validationFieldName(namespace string) string {
	elements := strings.Split(namespace, ".")
	if len(elements) == 1 {
		// An anonymous root struct has no type name in front of its fields.
		return topLevelName(namespace)
	}
	for _, element := range elements[1:] {
		if name := topLevelName(element); name != fieldNameHidden {
			return name
		}
	}
	return ""
}

// topLevelName reduces a dotted or indexed field path (files[0].content) to its first element (files).
func topLevelName(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// tagNamesOf collects the json and form tag names of t, embedded structs included. Non-struct types have none.
func tagNamesOf(t reflect.Type) map[string]bool {
	names := map[string]bool{}
	collectTagNames(t, names, map[reflect.Type]bool{})
	return names
}

func collectTagNames(t reflect.Type, names map[string]bool, visiting map[reflect.Type]bool) {
	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct || visiting[t] {
		return
	}
	visiting[t] = true
	for i := range t.NumField() {
		fld := t.Field(i)
		for _, tag := range []string{fld.Tag.Get("json"), fld.Tag.Get("form")} {
			if name := tagFieldName(tag); name != "" {
				names[name] = true
			}
		}
		if fld.Anonymous {
			collectTagNames(fld.Type, names, visiting)
		}
	}
	delete(visiting, t)
}

// respondInternalError answers 500 with publicText and logs the error text, sanitized, with the request logger. The
// answer never carries the error text itself, so internal messages (storage, encryption, driver) stay on the server.
func respondInternalError(c *gin.Context, publicText string, err error) {
	middleware.GetRequestLogger(c).
		WithField("error", util.SanitizeForLog(err.Error())).
		Error(publicText)
	c.JSON(http.StatusInternalServerError, gin.H{"error": publicText})
}
