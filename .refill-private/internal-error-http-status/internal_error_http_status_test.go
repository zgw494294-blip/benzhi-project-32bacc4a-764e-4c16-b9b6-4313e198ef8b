package internal_error_http_status_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/domain"
	"trapreview/internal/httpapi"
	"trapreview/internal/policy"
)

type failingRepository struct{}

func (failingRepository) Create(context.Context, *domain.Aggregate, string, json.RawMessage) (json.RawMessage, bool, error) {
	return nil, false, errors.New("storage unavailable")
}
func (failingRepository) Transact(context.Context, string, int64, string, application.Mutation) (json.RawMessage, bool, error) {
	return nil, false, errors.New("storage unavailable")
}
func (failingRepository) Load(context.Context, string) (*domain.Aggregate, error) {
	return nil, errors.New("storage unavailable")
}
func (failingRepository) LookupIdempotency(context.Context, string, string) (json.RawMessage, bool, error) {
	return nil, false, errors.New("storage unavailable")
}
func (failingRepository) FindRelease(context.Context, string) (*domain.DatasetRelease, error) {
	return nil, errors.New("storage unavailable")
}

func TestInternalRepositoryErrorReturnsServerError(t *testing.T) {
	service := application.NewService(failingRepository{}, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	handler := httpapi.NewHandler(service)
	malformedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/surveys", bytes.NewBufferString(`{"title":`))
	malformedResponse := httptest.NewRecorder()
	handler.ServeHTTP(malformedResponse, malformedRequest)
	if malformedResponse.Code != http.StatusBadRequest {
		t.Fatalf("畸形 JSON 应保持 400，status=%d body=%s", malformedResponse.Code, malformedResponse.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/surveys/survey-1", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("TestInternalRepositoryErrorReturnsServerError: status=%d body=%s", response.Code, response.Body.String())
	}
}
