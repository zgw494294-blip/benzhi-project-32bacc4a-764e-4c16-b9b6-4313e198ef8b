package store

import (
	"encoding/json"
	"fmt"
	"time"

	"trapreview/internal/domain"
)

func cloneAggregate(source *domain.Aggregate) (*domain.Aggregate, error) {
	if source == nil {
		return nil, fmt.Errorf("复制调查聚合: 源聚合为空")
	}
	target := *source
	target.Survey.SpeciesCatalog = append([]string(nil), source.Survey.SpeciesCatalog...)
	target.Stations = make(map[string]domain.CameraStation, len(source.Stations))
	for id, station := range source.Stations {
		station.RetrievedAt = cloneTimePointer(station.RetrievedAt)
		target.Stations[id] = station
	}
	target.Observations = make(map[string]domain.Observation, len(source.Observations))
	for id, observation := range source.Observations {
		observation.QualityFlags = append([]string(nil), observation.QualityFlags...)
		observation.LastVerifiedAt = cloneTimePointer(observation.LastVerifiedAt)
		target.Observations[id] = observation
	}
	target.Adjudications = append([]domain.Adjudication(nil), source.Adjudications...)
	target.Releases = make([]domain.DatasetRelease, len(source.Releases))
	for index, release := range source.Releases {
		clonedCounts := make(map[string]int, len(release.SpeciesCounts))
		for species, count := range release.SpeciesCounts {
			clonedCounts[species] = count
		}
		release.SpeciesCounts = clonedCounts
		release.Manifest.Items = append([]domain.ReleaseManifestItem(nil), release.Manifest.Items...)
		target.Releases[index] = release
	}
	target.AuditTrail = append([]domain.AuditEntry(nil), source.AuditTrail...)
	target.ApprovedAt = cloneTimePointer(source.ApprovedAt)
	return &target, nil
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func cloneRaw(source json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), source...) }

func idempotencyRef(scope, key string) string { return scope + "\x00" + key }
