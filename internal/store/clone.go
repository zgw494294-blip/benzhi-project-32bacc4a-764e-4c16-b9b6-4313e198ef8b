package store

import (
	"encoding/json"
	"fmt"

	"trapreview/internal/domain"
)

func cloneAggregate(source *domain.Aggregate) (*domain.Aggregate, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("复制调查聚合: %w", err)
	}
	var target domain.Aggregate
	if err := json.Unmarshal(data, &target); err != nil {
		return nil, fmt.Errorf("恢复调查聚合副本: %w", err)
	}
	if target.Stations == nil {
		target.Stations = map[string]domain.CameraStation{}
	}
	if target.Observations == nil {
		target.Observations = map[string]domain.Observation{}
	}
	return &target, nil
}

func cloneRaw(source json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), source...) }

func idempotencyRef(scope, key string) string { return scope + "\x00" + key }
