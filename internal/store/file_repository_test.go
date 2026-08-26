package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"trapreview/internal/domain"
)

func TestFileRepositoryRecoversEventsAndIdempotency(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	survey, _ := domain.NewSurvey("sur-store", "恢复测试", "甲", []string{"豹猫"}, now)
	aggregate := domain.NewAggregate(survey)
	created := json.RawMessage(`{"surveyId":"sur-store","expectedVersion":1}`)
	if _, replay, err := repository.Create(context.Background(), aggregate, "create-key", created); err != nil || replay {
		t.Fatalf("创建失败: replay=%v err=%v", replay, err)
	}
	result, replay, err := repository.Transact(context.Background(), survey.ID, 1, "station-key", func(working *domain.Aggregate) (json.RawMessage, error) {
		station, err := domain.NewCameraStation("station", survey.ID, "ST-01", "林缘", now, nil)
		if err != nil {
			return nil, err
		}
		if err := working.AddStation(station, "甲", now); err != nil {
			return nil, err
		}
		return json.RawMessage(`{"expectedVersion":2}`), nil
	})
	if err != nil || replay || len(result) == 0 {
		t.Fatalf("事务失败: %s replay=%v err=%v", result, replay, err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Load(context.Background(), survey.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Survey.ExpectedVersion != 2 || len(restored.Stations) != 1 {
		t.Fatalf("恢复状态错误: %+v", restored)
	}
	repeated, replay, err := reopened.Transact(context.Background(), survey.ID, 1, "station-key", func(*domain.Aggregate) (json.RawMessage, error) {
		t.Fatal("幂等重放不应再次调用变更函数")
		return nil, nil
	})
	if err != nil || !replay || string(repeated) != `{"expectedVersion":2}` {
		t.Fatalf("幂等重放错误: %s replay=%v err=%v", repeated, replay, err)
	}
}

func TestFileRepositoryRejectsStaleVersion(t *testing.T) {
	repository, _ := Open(t.TempDir())
	survey, _ := domain.NewSurvey("sur-version", "版本测试", "甲", []string{"豹猫"}, time.Now())
	_, _, _ = repository.Create(context.Background(), domain.NewAggregate(survey), "create", json.RawMessage(`{}`))
	_, _, err := repository.Transact(context.Background(), survey.ID, 99, "wrong", func(*domain.Aggregate) (json.RawMessage, error) { return nil, nil })
	if err == nil {
		t.Fatal("过期或超前版本应被拒绝")
	}
}
