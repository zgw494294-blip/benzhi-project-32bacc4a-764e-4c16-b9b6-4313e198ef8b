package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"trapreview/internal/domain"
)

type SubmitObservationCommand struct {
	SurveyID        string    `json:"-"`
	ExpectedVersion int64     `json:"expectedVersion"`
	StationID       string    `json:"stationId"`
	CapturedAt      time.Time `json:"capturedAt"`
	MediaRef        string    `json:"mediaRef"`
	MediaChecksum   string    `json:"mediaChecksum"`
	PrimaryLabel    string    `json:"primaryLabel"`
	SecondaryLabel  string    `json:"secondaryLabel"`
	Actor           string    `json:"actor"`
	IdempotencyKey  string    `json:"-"`
}

const maxObservationBatchSize = 100

type ObservationInput struct {
	StationID      string    `json:"stationId"`
	CapturedAt     time.Time `json:"capturedAt"`
	MediaRef       string    `json:"mediaRef"`
	MediaChecksum  string    `json:"mediaChecksum"`
	PrimaryLabel   string    `json:"primaryLabel"`
	SecondaryLabel string    `json:"secondaryLabel"`
}

type SubmitObservationBatchCommand struct {
	SurveyID        string             `json:"-"`
	ExpectedVersion int64              `json:"expectedVersion"`
	Items           []ObservationInput `json:"items"`
	Actor           string             `json:"actor"`
	IdempotencyKey  string             `json:"-"`
}

type ObservationBatchItemResult struct {
	InputIndex    int                      `json:"inputIndex"`
	ObservationID string                   `json:"observationId"`
	Status        domain.ObservationStatus `json:"status"`
}

type ObservationBatchResult struct {
	MutationResult
	BatchCount int                          `json:"batchCount"`
	Items      []ObservationBatchItemResult `json:"items"`
}

func (s *Service) SubmitObservationBatch(ctx context.Context, command SubmitObservationBatchCommand) (ObservationBatchResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return ObservationBatchResult{}, err
	}
	if len(command.Items) == 0 {
		return ObservationBatchResult{}, domain.Required("items")
	}
	if len(command.Items) > maxObservationBatchSize {
		return ObservationBatchResult{}, domain.Invalid("items", fmt.Sprintf("单批不能超过 %d 条", maxObservationBatchSize))
	}
	observations := make([]domain.Observation, len(command.Items))
	for index, item := range command.Items {
		observation, err := domain.NewObservation(newID("obs"), command.SurveyID, item.StationID, item.CapturedAt, item.MediaRef, item.MediaChecksum, item.PrimaryLabel, item.SecondaryLabel)
		if err != nil {
			return ObservationBatchResult{}, err
		}
		observations[index] = observation
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		for index, observation := range observations {
			if err := s.quality.ValidateSubmission(a.Survey, observation, fmt.Sprintf("items[%d]", index)); err != nil {
				return nil, err
			}
		}
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		if err := a.AddObservations(observations, actor, s.clock.Now()); err != nil {
			return nil, err
		}
		items := make([]ObservationBatchItemResult, len(observations))
		for index, observation := range observations {
			items[index] = ObservationBatchItemResult{InputIndex: index, ObservationID: observation.ID, Status: observation.Status}
		}
		return encodeResult(ObservationBatchResult{MutationResult: resultFrom(a, ""), BatchCount: len(items), Items: items})
	})
	if err != nil {
		return ObservationBatchResult{}, err
	}
	result, err := decodeResult[ObservationBatchResult](data)
	result.Replayed = replay
	return result, err
}

func (s *Service) SubmitObservation(ctx context.Context, command SubmitObservationCommand) (MutationResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	}
	observation, err := domain.NewObservation(newID("obs"), command.SurveyID, command.StationID, command.CapturedAt, command.MediaRef, command.MediaChecksum, command.PrimaryLabel, command.SecondaryLabel)
	if err != nil {
		return MutationResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		if err := a.AddObservation(observation, actor, s.clock.Now()); err != nil {
			return nil, err
		}
		return encodeResult(resultFrom(a, observation.ID))
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := decodeResult[MutationResult](data)
	result.Replayed = replay
	return result, err
}

type VerifySurveyCommand struct {
	SurveyID        string   `json:"-"`
	ExpectedVersion int64    `json:"expectedVersion"`
	ObservationIDs  []string `json:"observationIds,omitempty"`
	Actor           string   `json:"actor"`
	IdempotencyKey  string   `json:"-"`
}

func (s *Service) VerifySurvey(ctx context.Context, command VerifySurveyCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		if !a.Survey.Status.CanRecord() {
			return domain.InvalidState(a.Survey.Status, "核验观测")
		}
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = "system-verifier"
		}
		targets, err := verificationTargets(a, command.ObservationIDs)
		if err != nil {
			return err
		}
		results := make(map[string][]string, len(targets))
		for _, observation := range targets {
			results[observation.ID] = s.quality.Evaluate(a.Survey, observation)
		}
		return a.VerifyObservations(results, actor, s.clock.Now())
	})
}

func verificationTargets(a *domain.Aggregate, requested []string) ([]domain.Observation, error) {
	if len(requested) == 0 {
		targets := make([]domain.Observation, 0)
		for _, observation := range a.SortedObservations() {
			if observation.Status == domain.ObservationSubmitted {
				targets = append(targets, observation)
			}
		}
		return targets, nil
	}
	seen := make(map[string]struct{}, len(requested))
	ids := make([]string, 0, len(requested))
	for index, id := range requested {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, domain.Invalid(fmt.Sprintf("observationIds[%d]", index), "不能为空")
		}
		if _, exists := seen[id]; exists {
			return nil, domain.Invalid("observationIds", "包含重复观测标识")
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	targets := make([]domain.Observation, 0, len(ids))
	for _, id := range ids {
		observation, ok := a.Observations[id]
		if !ok {
			return nil, domain.NewError(domain.CodeNotFound, "观测不存在", "observationId")
		}
		if observation.Status == domain.ObservationSubmitted {
			targets = append(targets, observation)
		}
	}
	return targets, nil
}

type AdjudicateCommand struct {
	SurveyID        string                      `json:"-"`
	ObservationID   string                      `json:"-"`
	ExpectedVersion int64                       `json:"expectedVersion"`
	Reason          string                      `json:"reason"`
	Decision        domain.AdjudicationDecision `json:"decision"`
	ResolvedLabel   string                      `json:"resolvedLabel"`
	CorrectionNote  string                      `json:"correctionNote"`
	Adjudicator     string                      `json:"adjudicator"`
	IdempotencyKey  string                      `json:"-"`
}

func (s *Service) Adjudicate(ctx context.Context, command AdjudicateCommand) (MutationResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	}
	decision, err := domain.NewAdjudication(newID("adj"), command.SurveyID, command.ObservationID, command.Reason, command.Decision, command.ResolvedLabel, command.CorrectionNote, command.Adjudicator, s.clock.Now())
	if err != nil {
		return MutationResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		if command.Adjudicator == a.Survey.LeadResearcher {
			return nil, domain.Invalid("adjudicator", "必须独立于调查负责人")
		}
		if err := a.Adjudicate(decision, command.Adjudicator, s.clock.Now()); err != nil {
			return nil, err
		}
		return encodeResult(resultFrom(a, decision.ID))
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := decodeResult[MutationResult](data)
	result.Replayed = replay
	return result, err
}

type CorrectObservationCommand struct {
	SurveyID        string `json:"-"`
	ObservationID   string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	MediaRef        string `json:"mediaRef"`
	MediaChecksum   string `json:"mediaChecksum"`
	PrimaryLabel    string `json:"primaryLabel"`
	SecondaryLabel  string `json:"secondaryLabel"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"-"`
}

func (s *Service) CorrectObservation(ctx context.Context, command CorrectObservationCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		return a.CorrectObservation(command.ObservationID, command.MediaRef, command.MediaChecksum, command.PrimaryLabel, command.SecondaryLabel, actor, s.clock.Now())
	})
}
