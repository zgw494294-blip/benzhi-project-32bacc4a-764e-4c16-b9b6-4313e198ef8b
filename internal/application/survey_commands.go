package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"trapreview/internal/domain"
)

type CreateSurveyCommand struct {
	Title          string   `json:"title"`
	LeadResearcher string   `json:"leadResearcher"`
	SpeciesCatalog []string `json:"speciesCatalog"`
	IdempotencyKey string   `json:"-"`
}

func (s *Service) CreateSurvey(ctx context.Context, command CreateSurveyCommand) (MutationResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	}
	now := s.clock.Now()
	survey, err := domain.NewSurvey(newID("sur"), command.Title, command.LeadResearcher, command.SpeciesCatalog, now)
	if err != nil {
		return MutationResult{}, err
	}
	aggregate := domain.NewAggregate(survey)
	result := resultFrom(aggregate, survey.ID)
	encoded, err := encodeResult(result)
	if err != nil {
		return MutationResult{}, err
	}
	if data, ok, err := s.repository.LookupIdempotency(ctx, "create", command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	} else if ok {
		result, err := decodeResult[MutationResult](data)
		result.Replayed = true
		return result, err
	}
	stored, replayed, err := s.repository.Create(ctx, aggregate, command.IdempotencyKey, encoded)
	if err != nil {
		return MutationResult{}, err
	}
	if replayed {
		previous, err := decodeResult[MutationResult](stored)
		previous.Replayed = true
		return previous, err
	}
	return result, nil
}

type AddStationCommand struct {
	SurveyID            string     `json:"-"`
	ExpectedVersion     int64      `json:"expectedVersion"`
	StationCode         string     `json:"stationCode"`
	LocationDescription string     `json:"locationDescription"`
	DeployedAt          time.Time  `json:"deployedAt"`
	RetrievedAt         *time.Time `json:"retrievedAt,omitempty"`
	Actor               string     `json:"actor"`
	IdempotencyKey      string     `json:"-"`
}

type ReviseSurveyCommand struct {
	SurveyID        string   `json:"-"`
	ExpectedVersion int64    `json:"expectedVersion"`
	Title           string   `json:"title"`
	LeadResearcher  string   `json:"leadResearcher"`
	SpeciesCatalog  []string `json:"speciesCatalog"`
	Actor           string   `json:"actor"`
	IdempotencyKey  string   `json:"-"`
}

func (s *Service) ReviseSurvey(ctx context.Context, command ReviseSurveyCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		return a.ReviseSurvey(command.Title, command.LeadResearcher, command.SpeciesCatalog, actor, s.clock.Now())
	})
}

func (s *Service) AddStation(ctx context.Context, command AddStationCommand) (MutationResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	}
	station, err := domain.NewCameraStation(newID("sta"), command.SurveyID, command.StationCode, command.LocationDescription, command.DeployedAt, command.RetrievedAt)
	if err != nil {
		return MutationResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		if err := a.AddStation(station, actor, s.clock.Now()); err != nil {
			return nil, err
		}
		return encodeResult(resultFrom(a, station.ID))
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := decodeResult[MutationResult](data)
	result.Replayed = replay
	return result, err
}

type UpdateStationCommand struct {
	SurveyID            string     `json:"-"`
	StationID           string     `json:"-"`
	ExpectedVersion     int64      `json:"expectedVersion"`
	StationCode         string     `json:"stationCode"`
	LocationDescription string     `json:"locationDescription"`
	DeployedAt          time.Time  `json:"deployedAt"`
	RetrievedAt         *time.Time `json:"retrievedAt,omitempty"`
	Actor               string     `json:"actor"`
	IdempotencyKey      string     `json:"-"`
}

func (s *Service) UpdateStation(ctx context.Context, command UpdateStationCommand) (MutationResult, error) {
	if err := requireIdempotency(command.IdempotencyKey); err != nil {
		return MutationResult{}, err
	}
	station, err := domain.NewCameraStation(command.StationID, command.SurveyID, command.StationCode, command.LocationDescription, command.DeployedAt, command.RetrievedAt)
	if err != nil {
		return MutationResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) (json.RawMessage, error) {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		if err := a.UpdateStation(station, actor, s.clock.Now()); err != nil {
			return nil, err
		}
		return encodeResult(resultFrom(a, station.ID))
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := decodeResult[MutationResult](data)
	result.Replayed = replay
	return result, err
}

type RemoveStationCommand struct {
	SurveyID        string `json:"-"`
	StationID       string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"-"`
}

func (s *Service) RemoveStation(ctx context.Context, command RemoveStationCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		actor := strings.TrimSpace(command.Actor)
		if actor == "" {
			actor = a.Survey.LeadResearcher
		}
		return a.RemoveStation(command.StationID, actor, s.clock.Now())
	})
}

type LockProtocolCommand struct {
	SurveyID        string `json:"-"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"-"`
}

func (s *Service) LockProtocol(ctx context.Context, command LockProtocolCommand) (MutationResult, error) {
	return s.simpleMutation(ctx, command.SurveyID, command.ExpectedVersion, command.IdempotencyKey, func(a *domain.Aggregate) error {
		actor := command.Actor
		if strings.TrimSpace(actor) == "" {
			actor = a.Survey.LeadResearcher
		}
		return a.LockProtocol(actor, s.clock.Now())
	})
}

func (s *Service) simpleMutation(ctx context.Context, surveyID string, expected int64, key string, mutate func(*domain.Aggregate) error) (MutationResult, error) {
	if err := requireIdempotency(key); err != nil {
		return MutationResult{}, err
	}
	data, replay, err := s.repository.Transact(ctx, surveyID, expected, key, func(a *domain.Aggregate) (json.RawMessage, error) {
		if err := mutate(a); err != nil {
			return nil, err
		}
		return encodeResult(resultFrom(a, ""))
	})
	if err != nil {
		return MutationResult{}, err
	}
	result, err := decodeResult[MutationResult](data)
	result.Replayed = replay
	return result, err
}
