package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/policy"
	"trapreview/internal/store"
)

func TestCreateSurveyIsIdempotentAndRejectsUnknownFields(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	server := httptest.NewServer(NewHandler(service))
	defer server.Close()
	body := []byte(`{"title":"秦岭调查","leadResearcher":"调查员甲","speciesCatalog":["金丝猴"]}`)
	first := performRequest(t, server.Client(), server.URL+"/api/v1/surveys", "stable-key", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("首次创建状态码 %d", first.StatusCode)
	}
	var firstResult struct {
		Data application.MutationResult `json:"data"`
	}
	if err := json.NewDecoder(first.Body).Decode(&firstResult); err != nil {
		t.Fatal(err)
	}
	first.Body.Close()
	second := performRequest(t, server.Client(), server.URL+"/api/v1/surveys", "stable-key", body)
	if second.StatusCode != http.StatusOK {
		t.Fatalf("幂等重放状态码 %d", second.StatusCode)
	}
	var secondResult struct {
		Data application.MutationResult `json:"data"`
	}
	if err := json.NewDecoder(second.Body).Decode(&secondResult); err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if firstResult.Data.SurveyID != secondResult.Data.SurveyID || !secondResult.Data.Replayed {
		t.Fatalf("幂等结果不一致: %+v %+v", firstResult, secondResult)
	}
	invalid := performRequest(t, server.Client(), server.URL+"/api/v1/surveys", "other-key", []byte(`{"title":"调查","leadResearcher":"甲","speciesCatalog":["豹猫"],"unexpected":true}`))
	defer invalid.Body.Close()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知字段应返回 400，实际 %d", invalid.StatusCode)
	}
}

func performRequest(t *testing.T, client *http.Client, url, key string, body []byte) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
