package domain

import (
	"strings"
	"time"
)

type Observation struct {
	ID             string            `json:"id"`
	SurveyID       string            `json:"surveyId"`
	StationID      string            `json:"stationId"`
	CapturedAt     time.Time         `json:"capturedAt"`
	MediaRef       string            `json:"mediaRef"`
	MediaChecksum  string            `json:"mediaChecksum"`
	PrimaryLabel   string            `json:"primaryLabel"`
	SecondaryLabel string            `json:"secondaryLabel"`
	ResolvedLabel  string            `json:"resolvedLabel,omitempty"`
	QualityFlags   []string          `json:"qualityFlags"`
	Status         ObservationStatus `json:"status"`
	LastVerifiedAt *time.Time        `json:"lastVerifiedAt,omitempty"`
}

func NewObservation(id, surveyID, stationID string, captured time.Time, mediaRef, checksum, primary, secondary string) (Observation, error) {
	if strings.TrimSpace(stationID) == "" {
		return Observation{}, Required("stationId")
	}
	if captured.IsZero() {
		return Observation{}, Required("capturedAt")
	}
	return Observation{ID: id, SurveyID: surveyID, StationID: stationID, CapturedAt: captured.UTC(), MediaRef: strings.TrimSpace(mediaRef), MediaChecksum: strings.TrimSpace(checksum), PrimaryLabel: strings.TrimSpace(primary), SecondaryLabel: strings.TrimSpace(secondary), Status: ObservationSubmitted, QualityFlags: []string{}}, nil
}

func (o *Observation) ApplyVerification(flags []string, now time.Time) {
	o.QualityFlags = append([]string(nil), flags...)
	verifiedAt := now.UTC()
	o.LastVerifiedAt = &verifiedAt
	if len(flags) == 0 {
		o.Status = ObservationVerified
		if o.ResolvedLabel == "" {
			o.ResolvedLabel = o.PrimaryLabel
		}
	} else {
		o.Status = ObservationDisputed
	}
}

func (o *Observation) ApplyCorrection(mediaRef, checksum, primary, secondary string) {
	if mediaRef != "" {
		o.MediaRef = strings.TrimSpace(mediaRef)
	}
	if checksum != "" {
		o.MediaChecksum = strings.TrimSpace(checksum)
	}
	if primary != "" {
		o.PrimaryLabel = strings.TrimSpace(primary)
	}
	if secondary != "" {
		o.SecondaryLabel = strings.TrimSpace(secondary)
	}
	o.ResolvedLabel = ""
	o.QualityFlags = []string{}
	o.Status = ObservationSubmitted
	o.LastVerifiedAt = nil
}

func (o Observation) EffectiveLabel() string {
	if o.ResolvedLabel != "" {
		return o.ResolvedLabel
	}
	if o.PrimaryLabel == o.SecondaryLabel {
		return o.PrimaryLabel
	}
	return ""
}
