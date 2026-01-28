package models

import "time"

// CrowdsecConsoleEnrollment stores enrollment status and secrets for console registration.
type CrowdsecConsoleEnrollment struct {
	ID                 uint       `json:"-" gorm:"primaryKey"`
	UUID               string     `json:"uuid" gorm:"uniqueIndex"`
	Status             string     `json:"status" gorm:"index"`
	Tenant             string     `json:"tenant"`
	AgentName          string     `json:"agent_name"`
	EncryptedEnrollKey string     `json:"-" gorm:"type:text"`
	LastError          string     `json:"last_error" gorm:"type:text"`
	LastCorrelationID  string     `json:"last_correlation_id" gorm:"index"`
	LastAttemptAt      *time.Time `json:"last_attempt_at"`
	EnrolledAt         *time.Time `json:"enrolled_at"`
	LastHeartbeatAt    *time.Time `json:"last_heartbeat_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
