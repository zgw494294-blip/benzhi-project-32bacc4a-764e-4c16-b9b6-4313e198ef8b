package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func TestExtendedWorkflowThroughPublicHTTPAPI(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	server := httptest.NewServer(NewHandler(service))
	defer server.Close()

	var created application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, server.URL+"/api/v1/surveys", "ext-create", map[string]any{"title": "  初始调查  ", "leadResearcher": "调查员甲", "speciesCatalog": []string{"羚牛", "金丝猴"}}, http.StatusCreated, &created)

	var revised application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPatch, fmt.Sprintf("%s/api/v1/surveys/%s", server.URL, created.SurveyID), "ext-revise", map[string]any{"expectedVersion": created.ExpectedVersion, "title": " 修订调查 ", "leadResearcher": " 调查员乙 ", "speciesCatalog": []string{"金丝猴", "羚牛"}, "actor": "调查员甲"}, http.StatusOK, &revised)

	var station application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/stations", server.URL, created.SurveyID), "ext-station", map[string]any{"expectedVersion": revised.ExpectedVersion, "stationCode": "QL-01", "locationDescription": "北坡", "deployedAt": "2026-05-01T00:00:00Z"}, http.StatusOK, &station)

	var updated application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPut, fmt.Sprintf("%s/api/v1/surveys/%s/stations/%s", server.URL, created.SurveyID, station.ResourceID), "ext-station-update", map[string]any{"expectedVersion": station.ExpectedVersion, "stationCode": "QL-001", "locationDescription": "北坡林线", "deployedAt": "2026-05-01T00:00:00Z", "retrievedAt": "2026-05-10T00:00:00Z"}, http.StatusOK, &updated)

	var locked application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/lock", server.URL, created.SurveyID), "ext-lock", map[string]any{"expectedVersion": updated.ExpectedVersion}, http.StatusOK, &locked)

	batchBody := map[string]any{"expectedVersion": locked.ExpectedVersion, "actor": "调查员乙", "items": []map[string]any{
		{"stationId": station.ResourceID, "capturedAt": "2026-05-02T01:00:00Z", "mediaRef": "media://one", "mediaChecksum": checksumOf('a'), "primaryLabel": "金丝猴", "secondaryLabel": "金丝猴"},
		{"stationId": station.ResourceID, "capturedAt": "2026-05-03T01:00:00Z", "mediaRef": "media://two", "mediaChecksum": checksumOf('b'), "primaryLabel": "金丝猴", "secondaryLabel": "羚牛"},
	}}
	var batch application.ObservationBatchResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/batch", server.URL, created.SurveyID), "ext-batch", batchBody, http.StatusCreated, &batch)
	if batch.BatchCount != 2 || len(batch.Items) != 2 || batch.Items[0].InputIndex != 0 {
		t.Fatalf("批次结果不完整: %+v", batch)
	}
	var replay application.ObservationBatchResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/batch", server.URL, created.SurveyID), "ext-batch", batchBody, http.StatusOK, &replay)
	if !replay.Replayed || replay.Items[0].ObservationID != batch.Items[0].ObservationID {
		t.Fatalf("批次幂等重放不稳定: %+v", replay)
	}

	duplicateBody := map[string]any{"expectedVersion": batch.ExpectedVersion, "items": []map[string]any{{"stationId": station.ResourceID, "capturedAt": "2026-05-04T01:00:00Z", "mediaRef": "media://other", "mediaChecksum": checksumOf('a'), "primaryLabel": "羚牛", "secondaryLabel": "羚牛"}}}
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/batch", server.URL, created.SurveyID), "ext-duplicate", duplicateBody, http.StatusBadRequest, nil)

	var firstVerified application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/%s/verify", server.URL, created.SurveyID, batch.Items[0].ObservationID), "ext-verify-one", map[string]any{"expectedVersion": batch.ExpectedVersion, "actor": "核验员"}, http.StatusOK, &firstVerified)
	var secondVerified application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/verify", server.URL, created.SurveyID), "ext-verify-two", map[string]any{"expectedVersion": firstVerified.ExpectedVersion, "observationIds": []string{batch.Items[1].ObservationID}, "actor": "核验员"}, http.StatusOK, &secondVerified)

	var verification application.VerificationQueryResult
	requireExtensionStatus(t, server.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/surveys/%s/verification-results?status=verified&page=1&pageSize=1", server.URL, created.SurveyID), "", nil, http.StatusOK, &verification)
	if verification.Total != 1 || verification.LastVerificationTime == nil || len(verification.Items) != 1 {
		t.Fatalf("核验查询结果错误: %+v", verification)
	}

	var adjudicated application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/%s/adjudications", server.URL, created.SurveyID, batch.Items[1].ObservationID), "ext-adjudicate", map[string]any{"expectedVersion": secondVerified.ExpectedVersion, "reason": "双标注冲突", "decision": "accept_primary", "adjudicator": "复核员丙"}, http.StatusOK, &adjudicated)
	var reviewed application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/review", server.URL, created.SurveyID), "ext-review", map[string]any{"expectedVersion": adjudicated.ExpectedVersion}, http.StatusOK, &reviewed)
	var approved application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/approve", server.URL, created.SurveyID), "ext-approve", map[string]any{"expectedVersion": reviewed.ExpectedVersion, "reviewer": "复核员丙"}, http.StatusOK, &approved)
	var published application.PublishResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/releases", server.URL, created.SurveyID), "ext-publish", map[string]any{"expectedVersion": approved.ExpectedVersion, "actor": "数据负责人"}, http.StatusCreated, &published)
	if published.Release.ManifestDigest == "" || len(published.Release.Manifest.Items) != 2 {
		t.Fatalf("发布清单缺失: %+v", published.Release)
	}

	var contents application.ReleaseContentResult
	contentsURL := fmt.Sprintf("%s/api/v1/releases/verify/%s/contents?species=%s&page=1&pageSize=1", server.URL, published.Release.VerificationCode, url.QueryEscape("金丝猴"))
	requireExtensionStatus(t, server.Client(), http.MethodGet, contentsURL, "", nil, http.StatusOK, &contents)
	if contents.Total != 2 || len(contents.Items) != 1 || contents.ManifestDigest != published.Release.ManifestDigest {
		t.Fatalf("发布内容查询错误: %+v", contents)
	}

	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatalf("恢复包含发布清单的存储失败: %v", err)
	}
	restored, err := reopened.FindRelease(context.Background(), published.Release.VerificationCode)
	if err != nil || restored.ManifestDigest != published.Release.ManifestDigest {
		t.Fatalf("恢复后的发布凭据错误: release=%+v err=%v", restored, err)
	}
}

func TestRemovingLastDraftStationStillPreventsLock(t *testing.T) {
	repository, _ := store.Open(t.TempDir())
	server := httptest.NewServer(NewHandler(application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())))
	defer server.Close()
	var created application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, server.URL+"/api/v1/surveys", "remove-create", map[string]any{"title": "撤销测试", "leadResearcher": "甲", "speciesCatalog": []string{"豹猫"}}, http.StatusCreated, &created)
	var station application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/stations", server.URL, created.SurveyID), "remove-station", map[string]any{"expectedVersion": created.ExpectedVersion, "stationCode": "ST-1", "locationDescription": "林缘", "deployedAt": "2026-01-01T00:00:00Z"}, http.StatusOK, &station)
	var removed application.MutationResult
	requireExtensionStatus(t, server.Client(), http.MethodDelete, fmt.Sprintf("%s/api/v1/surveys/%s/stations/%s", server.URL, created.SurveyID, station.ResourceID), "remove-last", map[string]any{"expectedVersion": station.ExpectedVersion}, http.StatusOK, &removed)
	requireExtensionStatus(t, server.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/lock", server.URL, created.SurveyID), "remove-lock", map[string]any{"expectedVersion": removed.ExpectedVersion}, http.StatusBadRequest, nil)
}

func requireExtensionStatus(t *testing.T, client *http.Client, method, requestURL, key string, body any, expected int, target any) {
	t.Helper()
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, requestURL, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != expected {
		t.Fatalf("%s %s 状态码=%d，期望=%d，错误=%+v", method, requestURL, response.StatusCode, expected, envelope.Error)
	}
	if target != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			t.Fatal(err)
		}
	}
}

func checksumOf(value byte) string {
	data := make([]byte, 64)
	for index := range data {
		data[index] = value
	}
	return string(data)
}
