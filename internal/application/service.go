package application

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"trapreview/internal/domain"
	"trapreview/internal/policy"
)

type Service struct {
	repository Repository
	quality    policy.QualityPolicy
	release    policy.ReleasePolicy
	clock      Clock
}

func NewService(repository Repository, quality policy.QualityPolicy, release policy.ReleasePolicy) *Service {
	return &Service{repository: repository, quality: quality, release: release, clock: realClock{}}
}

func NewServiceWithClock(repository Repository, quality policy.QualityPolicy, release policy.ReleasePolicy, clock Clock) *Service {
	return &Service{repository: repository, quality: quality, release: release, clock: clock}
}

func newID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Sprintf("生成随机标识失败: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(value)
}

func verificationCode(surveyID string, version int64, salt string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", surveyID, version, salt)))
	return strings.ToUpper(hex.EncodeToString(digest[:16]))
}

func requireIdempotency(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.Required("idempotencyKey")
	}
	if len(key) > 128 {
		return domain.Invalid("idempotencyKey", "长度不能超过 128")
	}
	return nil
}

func encodeResult(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("编码应用结果: %w", err)
	}
	return data, nil
}

func decodeResult[T any](data json.RawMessage) (T, error) {
	var value T
	err := json.Unmarshal(data, &value)
	return value, err
}

type MutationResult struct {
	SurveyID        string              `json:"surveyId"`
	ExpectedVersion int64               `json:"expectedVersion"`
	Status          domain.SurveyStatus `json:"status"`
	ResourceID      string              `json:"resourceId,omitempty"`
	Replayed        bool                `json:"replayed,omitempty"`
}

func resultFrom(a *domain.Aggregate, resourceID string) MutationResult {
	return MutationResult{SurveyID: a.Survey.ID, ExpectedVersion: a.Survey.ExpectedVersion, Status: a.Survey.Status, ResourceID: resourceID}
}
