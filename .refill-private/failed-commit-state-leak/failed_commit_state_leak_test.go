package failed_commit_state_leak_test

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

func TestFailedCommitDoesNotLeakIntoRepositoryState(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	created, err := service.CreateSurvey(context.Background(), application.CreateSurveyCommand{
		Title:          "持久化失败隔离测试",
		LeadResearcher: "调查员甲",
		SpeciesCatalog: []string{"豹猫"},
		IdempotencyKey: "create-survey",
	})
	if err != nil {
		t.Fatalf("创建调查失败: %v", err)
	}

	eventsPath := filepath.Join(directory, "events.jsonl")
	backupPath := filepath.Join(directory, "events.backup")
	if err := os.Rename(eventsPath, backupPath); err != nil {
		t.Fatalf("移动事件日志失败: %v", err)
	}
	if err := os.Mkdir(eventsPath, 0o700); err != nil {
		t.Fatalf("制造事件日志写入故障失败: %v", err)
	}

	command := application.AddStationCommand{
		SurveyID:            created.SurveyID,
		ExpectedVersion:     created.ExpectedVersion,
		StationCode:         "ST-FAIL",
		LocationDescription: "林缘故障点位",
		DeployedAt:          time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC),
		Actor:               "调查员甲",
		IdempotencyKey:      "add-station-after-failure",
	}
	if _, err := service.AddStation(context.Background(), command); err == nil {
		t.Fatal("事件日志不可写时提交应返回错误")
	}

	detail, err := service.SurveyDetail(context.Background(), created.SurveyID)
	if err != nil {
		t.Fatalf("读取调查详情失败: %v", err)
	}
	if detail.Survey.ExpectedVersion != created.ExpectedVersion || len(detail.Stations) != 0 {
		t.Errorf("失败提交污染了内存状态: version=%d stations=%d", detail.Survey.ExpectedVersion, len(detail.Stations))
	}
	if result, err := service.AddStation(context.Background(), command); err == nil {
		t.Fatalf("失败提交被幂等缓存伪装成成功重放: replayed=%v version=%d", result.Replayed, result.ExpectedVersion)
	}
}
