package store

import (
	"encoding/json"
	"fmt"

	"trapreview/internal/domain"
)

func cloneAggregate(source *domain.Aggregate) (*domain.Aggregate, error) {
	if source == nil {
		return nil, fmt.Errorf("复制调查聚合: 源聚合为空")
	}
	target := *source
	target.Stations = make(map[string]domain.CameraStation, len(source.Stations))
	for id, station := range source.Stations {
		target.Stations[id] = station
	}
	target.Observations = make(map[string]domain.Observation, len(source.Observations))
	for id, observation := range source.Observations {
		target.Observations[id] = observation
	}
	target.Adjudications = append([]domain.Adjudication(nil), source.Adjudications...)
	target.Releases = append([]domain.DatasetRelease(nil), source.Releases...)
	target.AuditTrail = append([]domain.AuditEntry(nil), source.AuditTrail...)
	return &target, nil
}

func cloneRaw(source json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), source...) }

func idempotencyRef(scope, key string) string { return scope + "\x00" + key }
