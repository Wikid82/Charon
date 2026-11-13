package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type permissionRedactSample struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
	Count  int    `json:"count"`
}

func permissionRedactFlags(item permissionRedactSample) map[string]any {
	return map[string]any{"has_secret": item.Secret != ""}
}

func TestPermissionRedactForRoleAdminReceivesItemUnchanged(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "admin")
	item := permissionRedactSample{Name: "a", Secret: "s", Count: 2}

	got, err := redactForRole(ctx, item, []string{"secret"}, permissionRedactFlags)
	require.NoError(t, err)
	assert.Equal(t, item, got)
}

func TestPermissionRedactForRoleNonAdminLosesHiddenFieldsAndGetsFlags(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "user")
	item := permissionRedactSample{Name: "a", Secret: "s", Count: 2}

	got, err := redactForRole(ctx, item, []string{"secret"}, permissionRedactFlags)
	require.NoError(t, err)

	m, ok := got.(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, m, "secret")
	assert.Equal(t, "a", m["name"])
	assert.Equal(t, json.Number("2"), m["count"])
	assert.Equal(t, true, m["has_secret"])
}

func TestPermissionRedactForRoleNilFlagsAndNoRole(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	item := permissionRedactSample{Name: "a", Secret: "s"}

	got, err := redactForRole(ctx, item, []string{"secret"}, nil)
	require.NoError(t, err)
	m, ok := got.(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, m, "secret")
	assert.NotContains(t, m, "has_secret")
}

func TestPermissionRedactForRoleMarshalErrorIsReturned(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "user")

	_, err := redactForRole(ctx, make(chan int), nil, nil)
	require.Error(t, err)
}

func TestPermissionRedactListForRoleAdminReceivesItemsUnchanged(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "admin")
	items := []permissionRedactSample{{Name: "a", Secret: "s"}}

	got, err := redactListForRole(ctx, items, []string{"secret"}, permissionRedactFlags)
	require.NoError(t, err)
	assert.Equal(t, items, got)
}

func TestPermissionRedactListForRoleNonAdminRedactsEachItem(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "user")
	items := []permissionRedactSample{{Name: "a", Secret: "s"}, {Name: "b"}}

	got, err := redactListForRole(ctx, items, []string{"secret"}, permissionRedactFlags)
	require.NoError(t, err)

	list, ok := got.([]map[string]any)
	require.True(t, ok)
	require.Len(t, list, 2)
	assert.NotContains(t, list[0], "secret")
	assert.Equal(t, true, list[0]["has_secret"])
	assert.Equal(t, false, list[1]["has_secret"])
}

func TestPermissionRedactListForRoleEmptyListIsNotNull(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "user")

	got, err := redactListForRole[permissionRedactSample](ctx, nil, []string{"secret"}, nil)
	require.NoError(t, err)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(encoded))
}

func TestPermissionRedactListForRoleMarshalErrorIsReturned(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	ctx.Set("role", "user")

	_, err := redactListForRole(ctx, []chan int{make(chan int)}, nil, nil)
	require.Error(t, err)
}
