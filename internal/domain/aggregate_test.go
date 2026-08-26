package domain

import (
	"errors"
	"testing"
	"time"
)

func TestAggregateLifecycleRejectsEarlyApprovalAndDuplicatePublish(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	survey, err := NewSurvey("sur-1", "秦岭调查", "调查员甲", []string{"羚牛", "金丝猴"}, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := NewAggregate(survey)
	if err := aggregate.Approve("复核员乙", now); err == nil {
		t.Fatal("草稿状态不应允许批准")
	}
	station, err := NewCameraStation("sta-1", survey.ID, "QL-01", "北坡", now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := aggregate.AddStation(station, "调查员甲", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.LockProtocol("调查员甲", now); err != nil {
		t.Fatal(err)
	}
	observation, err := NewObservation("obs-1", survey.ID, station.ID, now.Add(time.Hour), "media://1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "金丝猴", "金丝猴")
	if err != nil {
		t.Fatal(err)
	}
	if err := aggregate.AddObservation(observation, "调查员甲", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.SetVerification(observation.ID, nil, "核验器", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.FinishVerification("核验器", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.RequestReview("调查员甲", now); err != nil {
		t.Fatal(err)
	}
	if err := aggregate.Approve("调查员甲", now); err == nil {
		t.Fatal("负责人不应批准自己的调查")
	}
	if err := aggregate.Approve("复核员乙", now); err != nil {
		t.Fatal(err)
	}
	release := DatasetRelease{ID: "rel-1", SurveyID: survey.ID, ReleaseNumber: 1, ObservationCount: 1, SpeciesCounts: map[string]int{"金丝猴": 1}, ApprovedBy: "复核员乙", IssuedAt: now, VerificationCode: "CODE"}
	if err := aggregate.Publish(release, "负责人丙", now); err != nil {
		t.Fatal(err)
	}
	err = aggregate.Publish(release, "负责人丙", now)
	var domainError *Error
	if !errors.As(err, &domainError) || domainError.Code != CodeDuplicateRelease {
		t.Fatalf("期望重复发布错误，实际 %v", err)
	}
}

func TestAggregateRequiresAdjudicationBeforeReview(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	survey, _ := NewSurvey("sur-2", "争议调查", "调查员甲", []string{"豹猫", "金猫"}, now)
	aggregate := NewAggregate(survey)
	station, _ := NewCameraStation("sta-2", survey.ID, "ST-2", "山脊", now, nil)
	_ = aggregate.AddStation(station, "调查员甲", now)
	_ = aggregate.LockProtocol("调查员甲", now)
	observation, _ := NewObservation("obs-2", survey.ID, station.ID, now, "media://2", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "豹猫", "金猫")
	_ = aggregate.AddObservation(observation, "调查员甲", now)
	_ = aggregate.SetVerification(observation.ID, []string{"label_conflict"}, "核验器", now)
	_ = aggregate.FinishVerification("核验器", now)
	if aggregate.Survey.Status != SurveyAdjudicating {
		t.Fatalf("期望 adjudicating，实际 %s", aggregate.Survey.Status)
	}
	if err := aggregate.RequestReview("调查员甲", now); err == nil {
		t.Fatal("未决争议不应允许申请复核")
	}
}
