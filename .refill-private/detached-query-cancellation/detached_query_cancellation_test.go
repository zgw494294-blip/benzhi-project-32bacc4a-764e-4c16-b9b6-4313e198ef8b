package detached_query_cancellation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/domain"
	"trapreview/internal/policy"
)

type cancellationRepository struct {
	aggregate *domain.Aggregate
	release   *domain.DatasetRelease
}

func (r *cancellationRepository) Load(ctx context.Context, _ string) (*domain.Aggregate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.aggregate, nil
}

func (r *cancellationRepository) FindRelease(ctx context.Context, _ string) (*domain.DatasetRelease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.release, nil
}

func (*cancellationRepository) Create(context.Context, *domain.Aggregate, string, json.RawMessage) (json.RawMessage, bool, error) {
	panic("unexpected Create call")
}

func (*cancellationRepository) Transact(context.Context, string, int64, string, application.Mutation) (json.RawMessage, bool, error) {
	panic("unexpected Transact call")
}

func (*cancellationRepository) LookupIdempotency(context.Context, string, string) (json.RawMessage, bool, error) {
	panic("unexpected LookupIdempotency call")
}

func TestCanceledQueryContextReachesRepository(t *testing.T) {
	aggregate := &domain.Aggregate{
		Survey:       domain.Survey{ID: "sur-canceled"},
		Stations:     map[string]domain.CameraStation{},
		Observations: map[string]domain.Observation{},
	}
	repository := &cancellationRepository{
		aggregate: aggregate,
		release: &domain.DatasetRelease{
			VerificationCode: "release-canceled",
			SpeciesCounts:    map[string]int{},
		},
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	queries := []struct {
		name string
		run  func() error
	}{
		{name: "survey detail", run: func() error {
			_, err := service.SurveyDetail(ctx, aggregate.Survey.ID)
			return err
		}},
		{name: "pending adjudications", run: func() error {
			_, err := service.PendingAdjudications(ctx, aggregate.Survey.ID)
			return err
		}},
		{name: "review summary", run: func() error {
			_, err := service.ReviewSummary(ctx, aggregate.Survey.ID)
			return err
		}},
		{name: "verification results", run: func() error {
			_, err := service.VerificationResults(ctx, application.VerificationQuery{SurveyID: aggregate.Survey.ID, Page: 1, PageSize: 50})
			return err
		}},
		{name: "verify release", run: func() error {
			_, err := service.VerifyRelease(ctx, repository.release.VerificationCode)
			return err
		}},
		{name: "release contents", run: func() error {
			_, err := service.ReleaseContents(ctx, application.ReleaseContentQuery{VerificationCode: repository.release.VerificationCode, Page: 1, PageSize: 50})
			return err
		}},
	}

	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			if err := query.run(); !errors.Is(err, context.Canceled) {
				t.Errorf("已取消的查询仍进入仓储并返回结果：err = %v，want context.Canceled", err)
			}
		})
	}
}
