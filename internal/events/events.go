package events

import (
	"time"

	"github.com/google/uuid"
	"github.com/noelukwa/indexer/internal/manager/models"
)

type NewIntent struct {
	RepoOwner string    `json:"repo_owner"`
	RepoName  string    `json:"repo_name"`
	From      time.Time `json:"from"`
	Until     time.Time `json:"until"`
	ID        uuid.UUID `json:"id"`
}

type NewCommitData struct {
	models.Commit
	Error *models.IntentError
}
