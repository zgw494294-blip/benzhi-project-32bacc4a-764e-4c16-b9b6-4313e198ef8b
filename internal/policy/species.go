package policy

import (
	"sort"
	"time"

	"trapreview/internal/domain"
)

func SpeciesCounts(aggregate *domain.Aggregate) map[string]int {
	counts := make(map[string]int)
	for _, observation := range aggregate.Observations {
		if observation.Status != domain.ObservationVerified {
			continue
		}
		if label := observation.EffectiveLabel(); label != "" {
			counts[label]++
		}
	}
	return counts
}

func BuildReleaseManifest(aggregate *domain.Aggregate, releaseNumber int, issuedAt time.Time) domain.ReleaseManifest {
	observations := aggregate.SortedObservations()
	sort.Slice(observations, func(i, j int) bool {
		if observations[i].CapturedAt.Equal(observations[j].CapturedAt) {
			return observations[i].ID < observations[j].ID
		}
		return observations[i].CapturedAt.Before(observations[j].CapturedAt)
	})
	items := make([]domain.ReleaseManifestItem, 0, len(observations))
	for _, observation := range observations {
		items = append(items, domain.ReleaseManifestItem{
			ObservationID: observation.ID,
			StationID:     observation.StationID,
			CapturedAt:    observation.CapturedAt.UTC(),
			MediaChecksum: observation.MediaChecksum,
			FinalSpecies:  observation.EffectiveLabel(),
		})
	}
	approvedAt := time.Time{}
	if aggregate.ApprovedAt != nil {
		approvedAt = aggregate.ApprovedAt.UTC()
	}
	return domain.ReleaseManifest{SurveyID: aggregate.Survey.ID, ReleaseNumber: releaseNumber, ApprovedBy: aggregate.ApprovedBy, ApprovedAt: approvedAt, IssuedAt: issuedAt.UTC(), Items: items}
}

type SpeciesCount struct {
	Species string `json:"species"`
	Count   int    `json:"count"`
}

func SortedSpeciesCounts(counts map[string]int) []SpeciesCount {
	result := make([]SpeciesCount, 0, len(counts))
	for species, count := range counts {
		result = append(result, SpeciesCount{Species: species, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Species < result[j].Species })
	return result
}
