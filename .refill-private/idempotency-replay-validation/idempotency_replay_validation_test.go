package idempotency_replay_validation_test

import (
	"context"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func TestIdempotentCreateReplaysBeforePayloadValidation(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	key := "create-replay-validation"
	first, err := service.CreateSurvey(context.Background(), application.CreateSurveyCommand{
		Title:          "秦岭红外调查",
		LeadResearcher: "调查员甲",
		SpeciesCatalog: []string{"金丝猴"},
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	second, err := service.CreateSurvey(context.Background(), application.CreateSurveyCommand{
		Title:          "",
		LeadResearcher: "",
		SpeciesCatalog: nil,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("相同幂等键应返回首次结果而不是重新校验请求: %v", err)
	}
	if !second.Replayed || second.SurveyID != first.SurveyID || second.ExpectedVersion != first.ExpectedVersion {
		t.Fatalf("幂等重放结果不一致: first=%+v second=%+v", first, second)
	}
}
