package resourcesavestatepollution_test

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

func TestSnapshotFailureRollbackKeepsMemoryAndDiskConsistent(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	created, err := service.CreateSurvey(context.Background(), application.CreateSurveyCommand{
		Title:          "快照回滚调查",
		LeadResearcher: "调查员甲",
		SpeciesCatalog: []string{"豹猫"},
		IdempotencyKey: "snapshot-create",
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(directory, "snapshot.json")
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshotPath, 0o700); err != nil {
		t.Fatal(err)
	}
	command := application.AddStationCommand{
		SurveyID:            created.SurveyID,
		ExpectedVersion:     created.ExpectedVersion,
		StationCode:         "ROLLBACK-01",
		LocationDescription: "林缘",
		DeployedAt:          time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		Actor:               "调查员甲",
		IdempotencyKey:      "snapshot-station",
	}
	if _, err := service.AddStation(context.Background(), command); err == nil {
		t.Fatal("快照替换失效时提交必须返回错误")
	}
	immediate, err := service.SurveyDetail(context.Background(), created.SurveyID)
	if err != nil {
		t.Fatal(err)
	}
	retry, retryErr := service.AddStation(context.Background(), command)

	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := application.NewService(reopened, policy.NewQualityPolicy(), policy.NewReleasePolicy()).SurveyDetail(context.Background(), created.SurveyID)
	if err != nil {
		t.Fatal(err)
	}

	if immediate.Survey.ExpectedVersion != created.ExpectedVersion || len(immediate.Stations) != 0 || retryErr == nil || retry.Replayed || restarted.Survey.ExpectedVersion != created.ExpectedVersion || len(restarted.Stations) != 0 {
		t.Fatalf("失败事务污染了内存或幂等状态: immediateVersion=%d immediateStations=%d retry=%+v retryErr=%v restartedVersion=%d restartedStations=%d",
			immediate.Survey.ExpectedVersion, len(immediate.Stations), retry, retryErr, restarted.Survey.ExpectedVersion, len(restarted.Stations))
	}
}
