package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"trapreview/internal/domain"
	"trapreview/internal/policy"
)

type SurveyDetail struct {
	Survey        domain.Survey           `json:"survey"`
	Stations      []domain.CameraStation  `json:"stations"`
	Observations  []domain.Observation    `json:"observations"`
	Adjudications []domain.Adjudication   `json:"adjudications"`
	Releases      []domain.DatasetRelease `json:"releases"`
	AuditTrail    []domain.AuditEntry     `json:"auditTrail"`
}

func (s *Service) SurveyDetail(ctx context.Context, surveyID string) (SurveyDetail, error) {
	a, err := s.repository.Load(ctx, surveyID)
	if err != nil {
		return SurveyDetail{}, fmt.Errorf("查询调查详情: %v", err)
	}
	detail := SurveyDetail{Survey: a.Survey, Stations: make([]domain.CameraStation, 0, len(a.Stations)), Observations: a.SortedObservations(), Adjudications: append([]domain.Adjudication(nil), a.Adjudications...), Releases: append([]domain.DatasetRelease(nil), a.Releases...), AuditTrail: append([]domain.AuditEntry(nil), a.AuditTrail...)}
	for _, station := range a.Stations {
		detail.Stations = append(detail.Stations, station)
	}
	sort.Slice(detail.Stations, func(i, j int) bool { return detail.Stations[i].StationCode < detail.Stations[j].StationCode })
	return detail, nil
}

func (s *Service) PendingAdjudications(ctx context.Context, surveyID string) ([]policy.PendingItem, error) {
	a, err := s.repository.Load(ctx, surveyID)
	if err != nil {
		return nil, fmt.Errorf("查询待裁定队列: %v", err)
	}
	return policy.PendingQueue(a), nil
}

func (s *Service) ReviewSummary(ctx context.Context, surveyID string) (policy.ReviewSummary, error) {
	a, err := s.repository.Load(ctx, surveyID)
	if err != nil {
		return policy.ReviewSummary{}, fmt.Errorf("查询复核摘要: %v", err)
	}
	return s.release.Summarize(a), nil
}

type VerificationResult struct {
	Valid   bool                   `json:"valid"`
	Release *domain.DatasetRelease `json:"release,omitempty"`
}

func (s *Service) VerifyRelease(ctx context.Context, code string) (VerificationResult, error) {
	release, err := s.repository.FindRelease(ctx, code)
	if err != nil {
		err = fmt.Errorf("核验发布凭据: %v", err)
		if e, ok := err.(*domain.Error); ok && e.Code == domain.CodeNotFound {
			return VerificationResult{Valid: false}, nil
		}
		return VerificationResult{}, err
	}
	return VerificationResult{Valid: true, Release: release}, nil
}

type VerificationQuery struct {
	SurveyID    string
	StationID   string
	Status      string
	QualityFlag string
	Page        int
	PageSize    int
}

type VerificationQueryResult struct {
	Items                []domain.Observation `json:"items"`
	Page                 int                  `json:"page"`
	PageSize             int                  `json:"pageSize"`
	Total                int                  `json:"total"`
	StatusCounts         map[string]int       `json:"statusCounts"`
	LastVerificationTime *time.Time           `json:"lastVerificationTime,omitempty"`
	BlockingReasons      []string             `json:"blockingReasons"`
}

func (s *Service) VerificationResults(ctx context.Context, query VerificationQuery) (VerificationQueryResult, error) {
	if err := validatePagination(query.Page, query.PageSize); err != nil {
		return VerificationQueryResult{}, err
	}
	if query.Status != "" && !validObservationStatus(query.Status) {
		return VerificationQueryResult{}, domain.Invalid("status", "不是有效观测状态")
	}
	if query.QualityFlag != "" && !policy.IsQualityFlag(query.QualityFlag) {
		return VerificationQueryResult{}, domain.Invalid("qualityFlag", "不是有效质量标记")
	}
	a, err := s.repository.Load(ctx, query.SurveyID)
	if err != nil {
		return VerificationQueryResult{}, fmt.Errorf("查询核验结果: %v", err)
	}
	filtered := make([]domain.Observation, 0)
	counts := map[string]int{
		string(domain.ObservationSubmitted):          0,
		string(domain.ObservationVerified):           0,
		string(domain.ObservationDisputed):           0,
		string(domain.ObservationCorrectionRequired): 0,
	}
	var last *time.Time
	for _, observation := range a.SortedObservations() {
		if query.StationID != "" && observation.StationID != query.StationID {
			continue
		}
		if query.Status != "" && string(observation.Status) != query.Status {
			continue
		}
		if query.QualityFlag != "" && !containsString(observation.QualityFlags, query.QualityFlag) {
			continue
		}
		filtered = append(filtered, observation)
		counts[string(observation.Status)]++
		if observation.LastVerifiedAt != nil && (last == nil || observation.LastVerifiedAt.After(*last)) {
			value := observation.LastVerifiedAt.UTC()
			last = &value
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CapturedAt.Equal(filtered[j].CapturedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CapturedAt.Before(filtered[j].CapturedAt)
	})
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	summary := s.release.Summarize(a)
	return VerificationQueryResult{Items: filtered[start:end], Page: query.Page, PageSize: query.PageSize, Total: len(filtered), StatusCounts: counts, LastVerificationTime: last, BlockingReasons: summary.BlockingReasons}, nil
}

type ReleaseContentQuery struct {
	VerificationCode string
	Species          string
	Page             int
	PageSize         int
}

type ReleaseContentResult struct {
	Items          []domain.ReleaseManifestItem `json:"items"`
	Page           int                          `json:"page"`
	PageSize       int                          `json:"pageSize"`
	Total          int                          `json:"total"`
	SpeciesCounts  map[string]int               `json:"speciesCounts"`
	ManifestDigest string                       `json:"manifestDigest"`
}

func (s *Service) ReleaseContents(ctx context.Context, query ReleaseContentQuery) (ReleaseContentResult, error) {
	if err := validatePagination(query.Page, query.PageSize); err != nil {
		return ReleaseContentResult{}, err
	}
	release, err := s.repository.FindRelease(ctx, query.VerificationCode)
	if err != nil {
		return ReleaseContentResult{}, fmt.Errorf("查询发布内容: %v", err)
	}
	species := strings.TrimSpace(query.Species)
	items := make([]domain.ReleaseManifestItem, 0, len(release.Manifest.Items))
	for _, item := range release.Manifest.Items {
		if species == "" || item.FinalSpecies == species {
			items = append(items, item)
		}
	}
	start := (query.Page - 1) * query.PageSize
	end := start + query.PageSize
	if start > len(items) {
		start = len(items)
	}
	if end > len(items) {
		end = len(items)
	}
	counts := make(map[string]int, len(release.SpeciesCounts))
	for label, count := range release.SpeciesCounts {
		counts[label] = count
	}
	return ReleaseContentResult{Items: items[start:end], Page: query.Page, PageSize: query.PageSize, Total: len(items), SpeciesCounts: counts, ManifestDigest: release.ManifestDigest}, nil
}

func validatePagination(page, pageSize int) error {
	if page < 1 {
		return domain.Invalid("page", "必须大于等于 1")
	}
	if pageSize < 1 || pageSize > 100 {
		return domain.Invalid("pageSize", "必须在 1 到 100 之间")
	}
	return nil
}

func validObservationStatus(value string) bool {
	switch domain.ObservationStatus(value) {
	case domain.ObservationSubmitted, domain.ObservationVerified, domain.ObservationDisputed, domain.ObservationCorrectionRequired:
		return true
	default:
		return false
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
