package cross_survey_commit_race_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"trapreview/internal/domain"
	"trapreview/internal/store"
)

func TestConcurrentSurveyCommitsRemainIsolated(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	for index := 1; index <= 2; index++ {
		id := fmt.Sprintf("sur-race-%d", index)
		survey, err := domain.NewSurvey(id, "并发提交隔离", "调查员", []string{"豹猫"}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := repository.Create(context.Background(), domain.NewAggregate(survey), fmt.Sprintf("create-%d", index), json.RawMessage(`{"expectedVersion":1}`)); err != nil {
			t.Fatal(err)
		}
	}

	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	for index := 1; index <= 2; index++ {
		index := index
		workers.Add(1)
		go func() {
			defer workers.Done()
			surveyID := fmt.Sprintf("sur-race-%d", index)
			_, _, err := repository.Transact(context.Background(), surveyID, 1, fmt.Sprintf("station-%d", index), func(working *domain.Aggregate) (json.RawMessage, error) {
				station, err := domain.NewCameraStation(fmt.Sprintf("station-%d", index), surveyID, fmt.Sprintf("ST-%02d", index), "林缘", now, nil)
				if err != nil {
					return nil, err
				}
				if err := working.AddStation(station, "调查员", now); err != nil {
					return nil, err
				}
				ready <- struct{}{}
				<-release
				return json.RawMessage(`{"expectedVersion":2}`), nil
			})
			errors <- err
		}()
	}

	readErrors := make(chan error, 2)
	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		<-release
		detail, err := repository.Load(context.Background(), "sur-race-1")
		if err == nil && detail.Survey.ExpectedVersion < 1 {
			err = fmt.Errorf("读取到无效调查版本 %d", detail.Survey.ExpectedVersion)
		}
		readErrors <- err
	}()
	go func() {
		defer readers.Done()
		<-release
		_, found, err := repository.LookupIdempotency(context.Background(), "create", "create-2")
		if err == nil && !found {
			err = fmt.Errorf("并发读取丢失已提交的幂等结果")
		}
		readErrors <- err
	}()

	<-ready
	<-ready
	close(release)
	workers.Wait()
	readers.Wait()
	close(errors)
	close(readErrors)
	for err := range errors {
		if err != nil {
			t.Fatalf("不同调查的并发提交应全部成功: %v", err)
		}
	}
	for err := range readErrors {
		if err != nil {
			t.Fatalf("提交期间的一致性读取失败: %v", err)
		}
	}

	for index := 1; index <= 2; index++ {
		detail, err := repository.Load(context.Background(), fmt.Sprintf("sur-race-%d", index))
		if err != nil {
			t.Fatal(err)
		}
		if detail.Survey.ExpectedVersion != 2 || len(detail.Stations) != 1 {
			t.Fatalf("调查 %d 的提交状态被其他调查污染: version=%d stations=%d", index, detail.Survey.ExpectedVersion, len(detail.Stations))
		}
	}
}
