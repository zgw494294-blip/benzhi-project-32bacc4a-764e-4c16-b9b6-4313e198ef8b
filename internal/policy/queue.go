package policy

import (
	"sort"

	"trapreview/internal/domain"
)

type PendingItem struct {
	Observation  domain.Observation   `json:"observation"`
	Reasons      []string             `json:"reasons"`
	LastDecision *domain.Adjudication `json:"lastDecision,omitempty"`
}

func PendingQueue(aggregate *domain.Aggregate) []PendingItem {
	latest := make(map[string]domain.Adjudication)
	for _, adjudication := range aggregate.Adjudications {
		latest[adjudication.ObservationID] = adjudication
	}
	items := make([]PendingItem, 0)
	for _, observation := range aggregate.Observations {
		if observation.Status != domain.ObservationDisputed && observation.Status != domain.ObservationCorrectionRequired {
			continue
		}
		item := PendingItem{Observation: observation, Reasons: append([]string(nil), observation.QualityFlags...)}
		if value, ok := latest[observation.ID]; ok {
			copied := value
			item.LastDecision = &copied
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Observation.CapturedAt.Equal(items[j].Observation.CapturedAt) {
			return items[i].Observation.ID < items[j].Observation.ID
		}
		return items[i].Observation.CapturedAt.Before(items[j].Observation.CapturedAt)
	})
	return items
}
