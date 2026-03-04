// Package trace provides request tracing context keys for correlating logs and metrics.
package trace

// ContextKey is a type alias for context keys used in request tracing.
type ContextKey string

// RequestIDKey is the context key for storing request IDs.
const RequestIDKey ContextKey = "requestID"
