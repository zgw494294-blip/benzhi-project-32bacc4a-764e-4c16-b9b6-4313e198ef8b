package detached_command_cancellation_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"trapreview/internal/application"
	"trapreview/internal/domain"
	"trapreview/internal/policy"
)

type cancellationProbeRepository struct {
	lookupContextErr   error
	mutationContextErr error
}

func (r *cancellationProbeRepository) Create(ctx context.Context, _ *domain.Aggregate, _ string, _ json.RawMessage) (json.RawMessage, bool, error) {
	r.mutationContextErr = ctx.Err()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return json.RawMessage(`{}`), false, nil
}

func (r *cancellationProbeRepository) Transact(ctx context.Context, _ string, _ int64, _ string, _ application.Mutation) (json.RawMessage, bool, error) {
	r.mutationContextErr = ctx.Err()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return json.RawMessage(`{}`), false, nil
}

func (r *cancellationProbeRepository) LookupIdempotency(ctx context.Context, _, _ string) (json.RawMessage, bool, error) {
	r.lookupContextErr = ctx.Err()
	return nil, false, nil
}

func (*cancellationProbeRepository) Load(context.Context, string) (*domain.Aggregate, error) {
	return nil, errors.New("unexpected Load call")
}

func (*cancellationProbeRepository) FindRelease(context.Context, string) (*domain.DatasetRelease, error) {
	return nil, errors.New("unexpected FindRelease call")
}

func TestCanceledCommandContextReachesRepository(t *testing.T) {
	deployedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		expectLookup bool
		run          func(context.Context, *application.Service) error
	}{
		{
			name:         "create survey",
			expectLookup: true,
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.CreateSurvey(ctx, application.CreateSurveyCommand{Title: "取消测试", LeadResearcher: "调查员甲", SpeciesCatalog: []string{"豹猫"}, IdempotencyKey: "cancel-create"})
				return err
			},
		},
		{
			name: "revise survey through simple mutation",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.ReviseSurvey(ctx, application.ReviseSurveyCommand{SurveyID: "sur-1", ExpectedVersion: 1, Title: "修订", LeadResearcher: "调查员甲", SpeciesCatalog: []string{"豹猫"}, IdempotencyKey: "cancel-revise"})
				return err
			},
		},
		{
			name: "add station",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.AddStation(ctx, application.AddStationCommand{SurveyID: "sur-1", ExpectedVersion: 1, StationCode: "ST-1", LocationDescription: "林缘", DeployedAt: deployedAt, IdempotencyKey: "cancel-add-station"})
				return err
			},
		},
		{
			name: "update station",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.UpdateStation(ctx, application.UpdateStationCommand{SurveyID: "sur-1", StationID: "sta-1", ExpectedVersion: 1, StationCode: "ST-1", LocationDescription: "林缘", DeployedAt: deployedAt, IdempotencyKey: "cancel-update-station"})
				return err
			},
		},
		{
			name: "submit observation batch",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.SubmitObservationBatch(ctx, application.SubmitObservationBatchCommand{SurveyID: "sur-1", ExpectedVersion: 1, Items: []application.ObservationInput{{StationID: "sta-1", CapturedAt: deployedAt}}, IdempotencyKey: "cancel-batch"})
				return err
			},
		},
		{
			name: "submit observation",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.SubmitObservation(ctx, application.SubmitObservationCommand{SurveyID: "sur-1", ExpectedVersion: 1, StationID: "sta-1", CapturedAt: deployedAt, IdempotencyKey: "cancel-observation"})
				return err
			},
		},
		{
			name: "adjudicate",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.Adjudicate(ctx, application.AdjudicateCommand{SurveyID: "sur-1", ObservationID: "obs-1", ExpectedVersion: 1, Reason: "标注冲突", Decision: domain.DecisionAcceptPrimary, Adjudicator: "复核员乙", IdempotencyKey: "cancel-adjudicate"})
				return err
			},
		},
		{
			name: "publish",
			run: func(ctx context.Context, service *application.Service) error {
				_, err := service.Publish(ctx, application.PublishCommand{SurveyID: "sur-1", ExpectedVersion: 1, IdempotencyKey: "cancel-publish"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &cancellationProbeRepository{}
			service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := test.run(ctx, service)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("已取消命令应返回 context.Canceled，实际为 %v", err)
			}
			if !errors.Is(repository.mutationContextErr, context.Canceled) {
				t.Fatalf("仓储 mutation 收到的 context 未取消：%v", repository.mutationContextErr)
			}
			if test.expectLookup && !errors.Is(repository.lookupContextErr, context.Canceled) {
				t.Fatalf("仓储幂等查询收到的 context 未取消：%v", repository.lookupContextErr)
			}
		})
	}
}
