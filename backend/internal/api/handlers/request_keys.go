package handlers

import (
	"maps"
	"slices"
)

// requestKeys returns the keys of a decoded request body, in no particular order. A handler passes them as the sent keys
// of a create, so the defaulted fields the client sent are stored as sent.
func requestKeys[V any](body map[string]V) []string {
	return slices.Collect(maps.Keys(body))
}
