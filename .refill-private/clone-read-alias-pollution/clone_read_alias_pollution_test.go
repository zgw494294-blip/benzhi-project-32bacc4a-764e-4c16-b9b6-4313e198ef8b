package clone_read_alias_pollution_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"trapreview/internal/application"
	"trapreview/internal/domain"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func TestSurveyDetailMutationDoesNotPolluteRepository(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(time.Hour)
	survey, err := domain.NewSurvey("sur-alias", "别名隔离调查", "调查员甲", []string{"豹猫", "金丝猴"}, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.NewAggregate(survey)
	aggregate.Observations["obs-alias"] = domain.Observation{
		ID:             "obs-alias",
		SurveyID:       survey.ID,
		QualityFlags:   []string{"label_conflict"},
		LastVerifiedAt: &verifiedAt,
		Status:         domain.ObservationDisputed,
	}
	aggregate.Releases = []domain.DatasetRelease{{
		SpeciesCounts: map[string]int{"豹猫": 1},
		Manifest: domain.ReleaseManifest{Items: []domain.ReleaseManifestItem{{
			ObservationID: "obs-alias",
			FinalSpecies:  "豹猫",
		}}},
	}}
	if _, _, err := repository.Create(context.Background(), aggregate, "create-alias-isolation", json.RawMessage(`{"created":true}`)); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())

	first, err := service.SurveyDetail(context.Background(), survey.ID)
	if err != nil {
		t.Fatal(err)
	}
	original := first.Survey.SpeciesCatalog[0]
	originalVerifiedAt := *first.Observations[0].LastVerifiedAt
	first.Survey.SpeciesCatalog[0] = "污染物种"
	first.Observations[0].QualityFlags[0] = "polluted_flag"
	*first.Observations[0].LastVerifiedAt = now.Add(48 * time.Hour)
	first.Releases[0].SpeciesCounts["豹猫"] = 99
	first.Releases[0].Manifest.Items[0].FinalSpecies = "污染物种"

	second, err := service.SurveyDetail(context.Background(), survey.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Survey.SpeciesCatalog[0] != original ||
		second.Observations[0].QualityFlags[0] != "label_conflict" ||
		!second.Observations[0].LastVerifiedAt.Equal(originalVerifiedAt) ||
		second.Releases[0].SpeciesCounts["豹猫"] != 1 ||
		second.Releases[0].Manifest.Items[0].FinalSpecies != "豹猫" {
		t.Fatalf("查询结果修改污染了仓储状态: catalog=%q flags=%v verifiedAt=%s speciesCounts=%v finalSpecies=%q",
			second.Survey.SpeciesCatalog[0], second.Observations[0].QualityFlags, second.Observations[0].LastVerifiedAt,
			second.Releases[0].SpeciesCounts, second.Releases[0].Manifest.Items[0].FinalSpecies)
	}
}
