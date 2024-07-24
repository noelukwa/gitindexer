package events

import (
	"time"

	"github.com/google/uuid"
	"github.com/noelukwa/indexer/internal/manager/models"
)

type CommitPayload struct {
	Commits []*models.Commit   `json:"commits"`
	Repo    *models.Repository `json:"repo"`
}

type CommitsEventKind string

const (
	NewCommitsKind  CommitsEventKind = "new_commits"
	NewRepoInfoKind CommitsEventKind = "new_repo_info"
)

type CommitsCommand struct {
	Kind    CommitsEventKind `json:"kind"`
	Payload *CommitPayload   `json:"paylad"`
}

type IntentPayload struct {
	RepoOwner string    `json:"repo_owner"`
	RepoName  string    `json:"repo_name"`
	From      time.Time `json:"from"`
	Until     time.Time `json:"until"`
	ID        uuid.UUID `json:"id"`
}

type IntentKind string

const (
	NewIntentKind    IntentKind = "new_intent"
	UpdateIntentKind IntentKind = "update_intent"
	CancelIntentKind IntentKind = "cancel_intent"
)

type IntentCommand struct {
	Kind   IntentKind     `json:"kind"`
	Intent *IntentPayload `json:"payload"`
}
