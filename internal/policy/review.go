package policy

import "trapreview/internal/domain"

type ReviewSummary struct {
	SurveyID           string   `json:"surveyId"`
	Status             string   `json:"status"`
	TotalObservations  int      `json:"totalObservations"`
	Verified           int      `json:"verified"`
	Disputed           int      `json:"disputed"`
	CorrectionRequired int      `json:"correctionRequired"`
	Submitted          int      `json:"submitted"`
	EvidenceIssues     int      `json:"evidenceIssues"`
	Completeness       float64  `json:"completeness"`
	Eligible           bool     `json:"eligible"`
	BlockingReasons    []string `json:"blockingReasons"`
}

type ReleasePolicy struct{}

func NewReleasePolicy() ReleasePolicy { return ReleasePolicy{} }

func (ReleasePolicy) Summarize(aggregate *domain.Aggregate) ReviewSummary {
	summary := ReviewSummary{SurveyID: aggregate.Survey.ID, Status: string(aggregate.Survey.Status), TotalObservations: len(aggregate.Observations), BlockingReasons: []string{}}
	for _, observation := range aggregate.Observations {
		switch observation.Status {
		case domain.ObservationVerified:
			summary.Verified++
		case domain.ObservationDisputed:
			summary.Disputed++
		case domain.ObservationCorrectionRequired:
			summary.CorrectionRequired++
		default:
			summary.Submitted++
		}
		for _, flag := range observation.QualityFlags {
			if IsEvidenceFlag(flag) {
				summary.EvidenceIssues++
				break
			}
		}
	}
	if summary.TotalObservations > 0 {
		summary.Completeness = float64(summary.Verified) / float64(summary.TotalObservations)
	}
	if summary.TotalObservations == 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "no_observations")
	}
	if summary.Disputed > 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "unresolved_disputes")
	}
	if summary.CorrectionRequired > 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "corrections_required")
	}
	if summary.Submitted > 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "unverified_observations")
	}
	if summary.EvidenceIssues > 0 {
		summary.BlockingReasons = append(summary.BlockingReasons, "evidence_issues")
	}
	summary.Eligible = len(summary.BlockingReasons) == 0 && summary.Completeness == 1
	return summary
}

func (p ReleasePolicy) EnsureEligible(aggregate *domain.Aggregate) error {
	summary := p.Summarize(aggregate)
	if summary.Eligible {
		return nil
	}
	if summary.Disputed+summary.CorrectionRequired > 0 {
		return domain.NewError(domain.CodeUnresolved, "调查仍有未解决争议", "observations")
	}
	return domain.NewError(domain.CodeValidation, "调查尚未满足发布完整度要求", "observations")
}
