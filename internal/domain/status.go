package domain

type SurveyStatus string

const (
	SurveyDraft          SurveyStatus = "draft"
	SurveyProtocolLocked SurveyStatus = "protocol_locked"
	SurveyRecording      SurveyStatus = "recording"
	SurveyAdjudicating   SurveyStatus = "adjudicating"
	SurveyReviewPending  SurveyStatus = "review_pending"
	SurveyApproved       SurveyStatus = "approved"
	SurveyPublished      SurveyStatus = "published"
)

type ObservationStatus string

const (
	ObservationSubmitted          ObservationStatus = "submitted"
	ObservationVerified           ObservationStatus = "verified"
	ObservationDisputed           ObservationStatus = "disputed"
	ObservationCorrectionRequired ObservationStatus = "correction_required"
)

type AdjudicationDecision string

const (
	DecisionAcceptPrimary     AdjudicationDecision = "accept_primary"
	DecisionAcceptSecondary   AdjudicationDecision = "accept_secondary"
	DecisionResolveLabel      AdjudicationDecision = "resolve_label"
	DecisionRequireCorrection AdjudicationDecision = "require_correction"
)

func (s SurveyStatus) IsTerminal() bool { return s == SurveyPublished }

func (s SurveyStatus) CanRecord() bool {
	return s == SurveyProtocolLocked || s == SurveyRecording || s == SurveyAdjudicating
}
