package caddy

import "errors"

// WalkDials calls visit for every "dial" string in a decoded JSON document,
// wherever it sits. The walk stops at the first error that visit returns.
func WalkDials(node any, visit func(dial string) error) error {
	switch v := node.(type) {
	case map[string]any:
		if dial, ok := v["dial"].(string); ok {
			if err := visit(dial); err != nil {
				return err
			}
		}
		for _, child := range v {
			if err := WalkDials(child, visit); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := WalkDials(child, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// HasDynamicUpstreams reports whether a decoded config sets dynamic upstreams
// anywhere. Their addresses are chosen at run time, so they cannot be checked.
func HasDynamicUpstreams(node any) bool {
	return hasKey(node, "dynamic_upstreams")
}

// HasTransportResolver reports whether a decoded config sets a custom resolver
// anywhere. A resolver decides which addresses a name reaches, so it is refused.
func HasTransportResolver(node any) bool {
	return hasKey(node, "resolver")
}

// errKeyFound stops the walk once a key has been seen.
var errKeyFound = errors.New("key found")

// hasKey reports whether any object under node has the key.
func hasKey(node any, key string) bool {
	err := walkObjects(node, func(obj map[string]any) error {
		if _, ok := obj[key]; ok {
			return errKeyFound
		}
		return nil
	})
	return errors.Is(err, errKeyFound)
}

// walkObjects calls visit for every JSON object under node. It stops at the first error.
func walkObjects(node any, visit func(map[string]any) error) error {
	switch v := node.(type) {
	case map[string]any:
		if err := visit(v); err != nil {
			return err
		}
		for _, child := range v {
			if err := walkObjects(child, visit); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := walkObjects(child, visit); err != nil {
				return err
			}
		}
	}
	return nil
}
