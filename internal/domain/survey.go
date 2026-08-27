package domain

import (
	"sort"
	"strings"
	"time"
)

type Survey struct {
	ID              string       `json:"id"`
	Title           string       `json:"title"`
	LeadResearcher  string       `json:"leadResearcher"`
	SpeciesCatalog  []string     `json:"speciesCatalog"`
	Status          SurveyStatus `json:"status"`
	ExpectedVersion int64        `json:"expectedVersion"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

func NewSurvey(id, title, lead string, catalog []string, now time.Time) (Survey, error) {
	if strings.TrimSpace(id) == "" {
		return Survey{}, Required("id")
	}
	if strings.TrimSpace(title) == "" {
		return Survey{}, Required("title")
	}
	if strings.TrimSpace(lead) == "" {
		return Survey{}, Required("leadResearcher")
	}
	clean, err := normalizeCatalog(catalog)
	if err != nil {
		return Survey{}, err
	}
	return Survey{ID: id, Title: strings.TrimSpace(title), LeadResearcher: strings.TrimSpace(lead), SpeciesCatalog: clean, Status: SurveyDraft, ExpectedVersion: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (s *Survey) Revise(title, lead string, catalog []string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return Required("title")
	}
	lead = strings.TrimSpace(lead)
	if lead == "" {
		return Required("leadResearcher")
	}
	clean, err := normalizeCatalog(catalog)
	if err != nil {
		return err
	}
	s.Title = title
	s.LeadResearcher = lead
	s.SpeciesCatalog = clean
	return nil
}

func normalizeCatalog(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, Required("speciesCatalog")
	}
	seen := make(map[string]struct{}, len(values))
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, Invalid("speciesCatalog", "包含空物种名")
		}
		if _, ok := seen[value]; ok {
			return nil, Invalid("speciesCatalog", "包含重复物种")
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	sort.Strings(clean)
	return clean, nil
}

func (s Survey) HasSpecies(label string) bool {
	i := sort.SearchStrings(s.SpeciesCatalog, label)
	return i < len(s.SpeciesCatalog) && s.SpeciesCatalog[i] == label
}
