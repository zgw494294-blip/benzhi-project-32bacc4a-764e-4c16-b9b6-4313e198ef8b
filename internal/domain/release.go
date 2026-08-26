package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type ReleaseManifestItem struct {
	ObservationID string    `json:"observationId"`
	StationID     string    `json:"stationId"`
	CapturedAt    time.Time `json:"capturedAt"`
	MediaChecksum string    `json:"mediaChecksum"`
	FinalSpecies  string    `json:"finalSpecies"`
}

type ReleaseManifest struct {
	SurveyID      string                `json:"surveyId"`
	ReleaseNumber int                   `json:"releaseNumber"`
	ApprovedBy    string                `json:"approvedBy"`
	ApprovedAt    time.Time             `json:"approvedAt"`
	IssuedAt      time.Time             `json:"issuedAt"`
	Items         []ReleaseManifestItem `json:"items"`
}

type DatasetRelease struct {
	ID               string          `json:"id"`
	SurveyID         string          `json:"surveyId"`
	ReleaseNumber    int             `json:"releaseNumber"`
	ObservationCount int             `json:"observationCount"`
	SpeciesCounts    map[string]int  `json:"speciesCounts"`
	ApprovedBy       string          `json:"approvedBy"`
	IssuedAt         time.Time       `json:"issuedAt"`
	VerificationCode string          `json:"verificationCode"`
	Manifest         ReleaseManifest `json:"manifest"`
	ManifestDigest   string          `json:"manifestDigest"`
}

func ManifestDigest(manifest ReleaseManifest) (string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("编码发布清单: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (r DatasetRelease) ValidateManifest() error {
	if r.Manifest.SurveyID != r.SurveyID || r.Manifest.ReleaseNumber != r.ReleaseNumber || r.Manifest.ApprovedBy != r.ApprovedBy || r.Manifest.ApprovedAt.IsZero() || !r.Manifest.IssuedAt.Equal(r.IssuedAt) {
		return fmt.Errorf("发布清单头信息与发布凭据不一致")
	}
	if len(r.Manifest.Items) != r.ObservationCount {
		return fmt.Errorf("发布清单记录数与发布凭据不一致")
	}
	digest, err := ManifestDigest(r.Manifest)
	if err != nil {
		return err
	}
	if digest != r.ManifestDigest {
		return fmt.Errorf("发布清单摘要不匹配")
	}
	return nil
}

type AuditEntry struct {
	Sequence int64     `json:"sequence"`
	Action   string    `json:"action"`
	Actor    string    `json:"actor"`
	Detail   string    `json:"detail"`
	At       time.Time `json:"at"`
}
