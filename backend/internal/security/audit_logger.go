// Package security provides audit logging for security-sensitive operations.
package security

import (
	"encoding/json"
	"log"
	"time"
)

// AuditEvent represents a security audit log entry.
// All fields are included in JSON output for structured logging.
type AuditEvent struct {
	Timestamp     string   `json:"timestamp"`      // RFC3339 timestamp of the event
	Action        string   `json:"action"`         // Action being performed (e.g., "url_validation", "url_test")
	Host          string   `json:"host"`           // Target hostname from URL
	RequestID     string   `json:"request_id"`     // Unique request identifier for tracing
	Result        string   `json:"result"`         // Result of action: "allowed", "blocked", "error"
	ResolvedIPs   []string `json:"resolved_ips"`   // DNS resolution results (for debugging)
	BlockedReason string   `json:"blocked_reason"` // Why the request was blocked
	UserID        string   `json:"user_id"`        // User who made the request (CRITICAL for attribution)
	SourceIP      string   `json:"source_ip"`      // IP address of the request originator
}

// AuditLogger provides structured security audit logging.
type AuditLogger struct {
	// prefix is prepended to all log messages
	prefix string
}

// NewAuditLogger creates a new security audit logger.
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{
		prefix: "[SECURITY AUDIT]",
	}
}

// LogURLValidation logs a URL validation event.
func (al *AuditLogger) LogURLValidation(event AuditEvent) {
	// Ensure timestamp is set
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Serialize to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("%s ERROR: Failed to serialize audit event: %v", al.prefix, err)
		return
	}

	// Log to standard logger (will be captured by application logger)
	log.Printf("%s %s", al.prefix, string(eventJSON))
}

// LogURLTest is a convenience method for logging URL connectivity tests.
func (al *AuditLogger) LogURLTest(host, requestID, userID, sourceIP, result string) {
	event := AuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    "url_connectivity_test",
		Host:      host,
		RequestID: requestID,
		Result:    result,
		UserID:    userID,
		SourceIP:  sourceIP,
	}
	al.LogURLValidation(event)
}

// LogSSRFBlock is a convenience method for logging blocked SSRF attempts.
func (al *AuditLogger) LogSSRFBlock(host string, resolvedIPs []string, reason, userID, sourceIP string) {
	event := AuditEvent{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Action:        "ssrf_block",
		Host:          host,
		ResolvedIPs:   resolvedIPs,
		BlockedReason: reason,
		Result:        "blocked",
		UserID:        userID,
		SourceIP:      sourceIP,
	}
	al.LogURLValidation(event)
}

// Global audit logger instance
var globalAuditLogger = NewAuditLogger()

// LogURLTest logs a URL test event using the global logger.
func LogURLTest(host, requestID, userID, sourceIP, result string) {
	globalAuditLogger.LogURLTest(host, requestID, userID, sourceIP, result)
}

// LogSSRFBlock logs a blocked SSRF attempt using the global logger.
func LogSSRFBlock(host string, resolvedIPs []string, reason, userID, sourceIP string) {
	globalAuditLogger.LogSSRFBlock(host, resolvedIPs, reason, userID, sourceIP)
}
