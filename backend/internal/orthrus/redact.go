package orthrus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// redactedValue replaces every environment value in a container inspect body.
const redactedValue = "[redacted]"

// maxInspectBodyBytes bounds how much of a container inspect body is buffered
// for redaction. Larger bodies fail closed (502).
const maxInspectBodyBytes = 8 << 20

// Sentinel failures. Their messages never include body content.
var (
	errInspectEncoded  = errors.New("inspect response is content-encoded")
	errInspectTooLarge = errors.New("inspect response exceeds the redaction limit")
	errInspectDecode   = errors.New("inspect response is not a redactable JSON object")
)

// redactEnvKey is the request-context key that marks a request whose response
// must have its environment values redacted. Muzzle sets it; redactInspectEnv
// reads it from the outgoing request.
type redactEnvKey struct{}

// isEnvRedacted reports whether ctx carries the redaction flag.
func isEnvRedacted(ctx context.Context) bool {
	v, _ := ctx.Value(redactEnvKey{}).(bool)
	return v
}

// isContainerInspect reports whether a normalised path is a container inspect.
// Only these responses carry Config.Env.
func isContainerInspect(stripped string) bool {
	matched, err := path.Match("/containers/*/json", stripped)
	return err == nil && matched
}

// redactContainerEnv replaces the value of every Config.Env entry with
// redactedValue, keeping the name. Entries without "=" are left as they are.
// Every other field of the document is preserved. A document with no
// Config.Env (or a null one) is returned unchanged.
func redactContainerEnv(body []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, fmt.Errorf("orthrus: decode container inspect: %w", errInspectDecode)
	}
	rawConfig, ok := top["Config"]
	if !ok {
		return body, nil
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(rawConfig, &config); err != nil {
		return nil, fmt.Errorf("orthrus: decode container config: %w", errInspectDecode)
	}
	rawEnv, ok := config["Env"]
	if !ok {
		return body, nil
	}

	var env []string
	if err := json.Unmarshal(rawEnv, &env); err != nil {
		return nil, fmt.Errorf("orthrus: decode container env: %w", errInspectDecode)
	}
	if len(env) == 0 {
		return body, nil
	}
	for i, entry := range env {
		if name, _, found := strings.Cut(entry, "="); found {
			env[i] = name + "=" + redactedValue
		}
	}

	newEnv, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("orthrus: encode redacted env: %w", err)
	}
	config["Env"] = newEnv
	newConfig, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("orthrus: encode redacted config: %w", err)
	}
	top["Config"] = newConfig
	out, err := json.Marshal(top)
	if err != nil {
		return nil, fmt.Errorf("orthrus: encode redacted inspect: %w", err)
	}
	return out, nil
}

// redactInspectEnv is the ReverseProxy ModifyResponse hook for the external
// proxy. It acts only on a response whose request carries the redaction flag
// and whose status is 200. Any failure returns an error, so the proxy answers
// 502 and never forwards an unredacted body.
func redactInspectEnv(resp *http.Response) error {
	if resp.Request == nil || !isEnvRedacted(resp.Request.Context()) {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	if resp.Header.Get("Content-Encoding") != "" {
		return fmt.Errorf("orthrus: redact inspect: %w", errInspectEncoded)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxInspectBodyBytes+1))
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("orthrus: read inspect response: %w", err)
	}
	if len(raw) > maxInspectBodyBytes {
		return fmt.Errorf("orthrus: redact inspect: %w", errInspectTooLarge)
	}

	redacted, err := redactContainerEnv(raw)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(redacted))
	resp.ContentLength = int64(len(redacted))
	resp.Header.Set("Content-Length", strconv.Itoa(len(redacted)))
	resp.Header.Del("Transfer-Encoding")
	return nil
}
