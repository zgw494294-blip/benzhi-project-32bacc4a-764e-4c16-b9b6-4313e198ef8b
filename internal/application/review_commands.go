package application

import (
	"context"
	"encoding/json"
	"strings"

	"trapreview/internal/domain"
	"trapreview/internal/policy"
)

type RequestReviewCommand struct {
	SurveyID        string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"-"`
}

func (s *Service) RequestReview(ctx context.Context, command RequestReviewCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		if err := s.release.EnsureEligible(a); err != nil {
			return err
		}
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		return a.RequestReview(actor, s.clock.Now())
	})
}

type ApproveCommand struct {
	SurveyID        string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Reviewer        string `json:"reviewer"`
	IdempotencyKey  string `json:"-"`
}

func (s *Service) Approve(ctx context.Context, command ApproveCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		if err := s.release.EnsureEligible(a); err != nil {
			return err
		}
		return a.Approve(command.Reviewer, s.clock.Now())
	})
}

type PublishCommand struct {
	SurveyID        string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"-"`
}

type PublishResult struct {
	MutationResult
	Release domain.DatasetRelease `json:"release"`
}

func (s *Service) Publish(ctx context.Context, command PublishCommand) (PublishResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return PublishResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		if err := s.release.EnsureEligible(a); err != nil {
			return nil, err
		}
		now := s.clock.Now()
		manifest := policy.BuildReleaseManifest(a, 1, now)
		digest, err := domain.ManifestDigest(manifest)
		if err != nil {
			return nil, err
		}
		release := domain.DatasetRelease{ID: newID("rel"), SurveyID: a.Survey.ID, ReleaseNumber: 1, ObservationCount: len(a.Observations), SpeciesCounts: policy.SpeciesCounts(a), ApprovedBy: a.ApprovedBy, IssuedAt: now, VerificationCode: verificationCode(a.Survey.ID, a.Survey.ExpectedVersion, command.IdempotencyKey), Manifest: manifest, ManifestDigest: digest}
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.ApprovedBy
		}
		if err := a.Publish(release, actor, now); err != nil {
			return nil, err
		}
		return encodeResult(PublishResult{MutationResult: resultFrom(a, release.ID), Release: release})
	})
	if err != nil {
		return PublishResult{}, err
	}
	result, err := decodeResult[PublishResult](data)
	result.Replayed = replay
	return result, err
}
