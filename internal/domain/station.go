package domain

import (
	"strings"
	"time"
)

type CameraStation struct {
	ID                  string     `json:"id"`
	SurveyID            string     `json:"surveyId"`
	StationCode         string     `json:"stationCode"`
	LocationDescription string     `json:"locationDescription"`
	DeployedAt          time.Time  `json:"deployedAt"`
	RetrievedAt         *time.Time `json:"retrievedAt,omitempty"`
	Active              bool       `json:"active"`
}

func NewCameraStation(id, surveyID, code, location string, deployed time.Time, retrieved *time.Time) (CameraStation, error) {
	if strings.TrimSpace(code) == "" {
		return CameraStation{}, Required("stationCode")
	}
	if strings.TrimSpace(location) == "" {
		return CameraStation{}, Required("locationDescription")
	}
	if deployed.IsZero() {
		return CameraStation{}, Required("deployedAt")
	}
	if retrieved != nil && retrieved.Before(deployed) {
		return CameraStation{}, Invalid("retrievedAt", "不能早于 deployedAt")
	}
	active := retrieved == nil
	return CameraStation{ID: id, SurveyID: surveyID, StationCode: strings.TrimSpace(code), LocationDescription: strings.TrimSpace(location), DeployedAt: deployed.UTC(), RetrievedAt: utcPointer(retrieved), Active: active}, nil
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	converted := value.UTC()
	return &converted
}

func (s CameraStation) Contains(captured time.Time) bool {
	if captured.Before(s.DeployedAt) {
		return false
	}
	return s.RetrievedAt == nil || !captured.After(*s.RetrievedAt)
}
