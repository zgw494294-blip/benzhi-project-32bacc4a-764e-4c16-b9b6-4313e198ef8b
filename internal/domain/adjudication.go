package domain

import (
	"strings"
	"time"
)

type Adjudication struct {
	ID             string               `json:"id"`
	SurveyID       string               `json:"surveyId"`
	ObservationID  string               `json:"observationId"`
	Reason         string               `json:"reason"`
	Decision       AdjudicationDecision `json:"decision"`
	ResolvedLabel  string               `json:"resolvedLabel,omitempty"`
	CorrectionNote string               `json:"correctionNote,omitempty"`
	Adjudicator    string               `json:"adjudicator"`
	DecidedAt      time.Time            `json:"decidedAt"`
}

func NewAdjudication(id, surveyID, observationID, reason string, decision AdjudicationDecision, resolved, note, adjudicator string, now time.Time) (Adjudication, error) {
	if strings.TrimSpace(reason) == "" {
		return Adjudication{}, Required("reason")
	}
	if strings.TrimSpace(adjudicator) == "" {
		return Adjudication{}, Required("adjudicator")
	}
	switch decision {
	case DecisionAcceptPrimary, DecisionAcceptSecondary:
		if resolved != "" {
			return Adjudication{}, Invalid("resolvedLabel", "接受原标注时不得另外指定")
		}
	case DecisionResolveLabel:
		if strings.TrimSpace(resolved) == "" {
			return Adjudication{}, Required("resolvedLabel")
		}
	case DecisionRequireCorrection:
		if strings.TrimSpace(note) == "" {
			return Adjudication{}, Required("correctionNote")
		}
	default:
		return Adjudication{}, Invalid("decision", "不是支持的裁定决定")
	}
	return Adjudication{ID: id, SurveyID: surveyID, ObservationID: observationID, Reason: strings.TrimSpace(reason), Decision: decision, ResolvedLabel: strings.TrimSpace(resolved), CorrectionNote: strings.TrimSpace(note), Adjudicator: strings.TrimSpace(adjudicator), DecidedAt: now.UTC()}, nil
}
