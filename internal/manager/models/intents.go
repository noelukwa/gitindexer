package models

import (
	"time"

	"github.com/google/uuid"
)

type IntentStatus string

const (
	PendingBroadCast IntentStatus = "pending_broadcast"
	SuccessBroadCast IntentStatus = "success_broadcast"
)

type Intent struct {
	RepositoryName string       `json:"repository_name"`
	StartDate      time.Time    `json:"start_date"`
	Until          time.Time    `json:"end_date"`
	Status         IntentStatus `json:"status"`
	IsActive       bool         `json:"is_active"`
	Error          *IntentError `json:"error,omitempty"`
	ID             uuid.UUID    `json:"id"`
}

type IntentUpdate struct {
	ID        uuid.UUID
	Status    *IntentStatus `json:"status"`
	IsActive  *bool         `json:"is_active"`
	StartDate *time.Time    `json:"start_date"`
}

type IntentError struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
	IntentID  uuid.UUID
}

type IntentFilter struct {
	Status         *IntentStatus `json:"status"`
	IsActive       *bool         `json:"is_active"`
	RepositoryName *string       `json:"repository_name"`
}
