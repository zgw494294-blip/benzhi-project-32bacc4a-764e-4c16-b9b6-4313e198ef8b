package store

import (
	"encoding/json"
	"time"

	"trapreview/internal/domain"
)

const schemaVersion = 1

type snapshot struct {
	SchemaVersion int                              `json:"schemaVersion"`
	LastSequence  int64                            `json:"lastSequence"`
	Surveys       map[string]*domain.Aggregate     `json:"surveys"`
	Idempotency   map[string]json.RawMessage       `json:"idempotency"`
	Releases      map[string]domain.DatasetRelease `json:"releases"`
}

type eventRecord struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Sequence       int64             `json:"sequence"`
	Kind           string            `json:"kind"`
	SurveyID       string            `json:"surveyId"`
	SurveyVersion  int64             `json:"surveyVersion"`
	IdempotencyRef string            `json:"idempotencyRef"`
	Result         json.RawMessage   `json:"result"`
	Aggregate      *domain.Aggregate `json:"aggregate"`
	RecordedAt     time.Time         `json:"recordedAt"`
	Checksum       string            `json:"checksum"`
}

func emptySnapshot() snapshot {
	return snapshot{SchemaVersion: schemaVersion, Surveys: map[string]*domain.Aggregate{}, Idempotency: map[string]json.RawMessage{}, Releases: map[string]domain.DatasetRelease{}}
}
