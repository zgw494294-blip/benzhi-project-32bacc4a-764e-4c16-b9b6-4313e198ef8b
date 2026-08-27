package policy

import (
	"slices"
	"testing"
	"time"

	"trapreview/internal/domain"
)

func TestQualityPolicyFindsEvidenceCatalogAndLabelProblems(t *testing.T) {
	survey, err := domain.NewSurvey("sur-1", "测试调查", "甲", []string{"金丝猴", "羚牛"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	observation, err := domain.NewObservation("obs-1", survey.ID, "station", time.Now(), "", "short", "金丝猴", "大熊猫")
	if err != nil {
		t.Fatal(err)
	}
	flags := NewQualityPolicy().Evaluate(survey, observation)
	for _, expected := range []string{FlagMissingMediaRef, FlagInvalidMediaChecksum, FlagSecondaryOutOfCatalog, FlagLabelConflict} {
		if !slices.Contains(flags, expected) {
			t.Errorf("缺少质量标记 %s: %v", expected, flags)
		}
	}
}

func TestReleaseSummaryIsDeterministic(t *testing.T) {
	survey, _ := domain.NewSurvey("sur-2", "测试调查", "甲", []string{"豹猫"}, time.Now())
	aggregate := domain.NewAggregate(survey)
	aggregate.Observations["verified"] = domain.Observation{ID: "verified", Status: domain.ObservationVerified}
	aggregate.Observations["disputed"] = domain.Observation{ID: "disputed", Status: domain.ObservationDisputed, QualityFlags: []string{FlagMissingMediaRef}}
	summary := NewReleasePolicy().Summarize(aggregate)
	if summary.Completeness != 0.5 || summary.Eligible || summary.EvidenceIssues != 1 {
		t.Fatalf("审核摘要不符合预期: %+v", summary)
	}
}
