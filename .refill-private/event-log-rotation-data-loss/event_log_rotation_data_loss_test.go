package eventlogrotationdataloss_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trapreview/internal/application"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func TestRotatedEventLogKeepsCommittedSurveyState(t *testing.T) {
	dataDir := t.TempDir()
	repository, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	ctx := context.Background()
	created, err := service.CreateSurvey(ctx, application.CreateSurveyCommand{
		Title:          "事件日志轮换恢复测试",
		LeadResearcher: "调查员甲",
		SpeciesCatalog: []string{"豹猫"},
		IdempotencyKey: "create-before-rotation",
	})
	if err != nil {
		t.Fatalf("创建调查失败: %v", err)
	}

	eventsPath := filepath.Join(dataDir, "events.jsonl")
	baseline, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("读取轮换前事件日志失败: %v", err)
	}
	if err := os.Rename(eventsPath, eventsPath+".rotated"); err != nil {
		t.Fatalf("轮换事件日志失败: %v", err)
	}
	if err := os.WriteFile(eventsPath, baseline, 0o600); err != nil {
		t.Fatalf("建立新事件日志失败: %v", err)
	}

	result, err := service.AddStation(ctx, application.AddStationCommand{
		SurveyID:            created.SurveyID,
		ExpectedVersion:     created.ExpectedVersion,
		StationCode:         "ROT-01",
		LocationDescription: "林缘轮换点位",
		DeployedAt:          time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC),
		Actor:               "调查员甲",
		IdempotencyKey:      "station-after-rotation",
	})
	if err != nil {
		t.Fatalf("轮换后提交点位失败: %v", err)
	}
	if result.ExpectedVersion != 2 {
		t.Fatalf("轮换后提交版本错误: got %d want 2", result.ExpectedVersion)
	}

	reopened, err := store.Open(dataDir)
	if err != nil {
		t.Fatalf("重启恢复仓储失败: %v", err)
	}
	restored, err := reopened.Load(ctx, created.SurveyID)
	if err != nil {
		t.Fatalf("重启后读取调查失败: %v", err)
	}
	if restored.Survey.ExpectedVersion != 2 || len(restored.Stations) != 1 {
		t.Fatalf("重启后提交状态丢失: version=%d stations=%d", restored.Survey.ExpectedVersion, len(restored.Stations))
	}
}
