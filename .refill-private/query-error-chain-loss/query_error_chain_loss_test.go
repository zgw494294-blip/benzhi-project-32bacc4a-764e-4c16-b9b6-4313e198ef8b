package queryerrorchainloss_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trapreview/internal/application"
	"trapreview/internal/domain"
	"trapreview/internal/httpapi"
	"trapreview/internal/policy"
)

type missingRepository struct{}

func (missingRepository) Create(context.Context, *domain.Aggregate, string, json.RawMessage) (json.RawMessage, bool, error) {
	return nil, false, fmt.Errorf("unexpected Create call")
}

func (missingRepository) Transact(context.Context, string, int64, string, application.Mutation) (json.RawMessage, bool, error) {
	return nil, false, fmt.Errorf("unexpected Transact call")
}

func (missingRepository) Load(context.Context, string) (*domain.Aggregate, error) {
	return nil, domain.NewError(domain.CodeNotFound, "调查任务不存在", "surveyId")
}

func (missingRepository) LookupIdempotency(context.Context, string, string) (json.RawMessage, bool, error) {
	return nil, false, fmt.Errorf("unexpected LookupIdempotency call")
}

func (missingRepository) FindRelease(context.Context, string) (*domain.DatasetRelease, error) {
	return nil, domain.NewError(domain.CodeNotFound, "发布凭据不存在", "verificationCode")
}

func TestQueryRepositoryErrorsPreserveHTTPClassification(t *testing.T) {
	service := application.NewService(missingRepository{}, policy.NewQualityPolicy(), policy.NewReleasePolicy())
	handler := httpapi.NewHandler(service)
	tests := []struct {
		name       string
		path       string
		statusCode int
		errorCode  string
	}{
		{name: "survey detail", path: "/api/v1/surveys/missing", statusCode: http.StatusNotFound, errorCode: string(domain.CodeNotFound)},
		{name: "pending adjudications", path: "/api/v1/surveys/missing/adjudications/pending", statusCode: http.StatusNotFound, errorCode: string(domain.CodeNotFound)},
		{name: "review summary", path: "/api/v1/surveys/missing/review-summary", statusCode: http.StatusNotFound, errorCode: string(domain.CodeNotFound)},
		{name: "verification results", path: "/api/v1/surveys/missing/verification-results", statusCode: http.StatusNotFound, errorCode: string(domain.CodeNotFound)},
		{name: "release contents", path: "/api/v1/releases/verify/missing/contents", statusCode: http.StatusNotFound, errorCode: string(domain.CodeNotFound)},
		{name: "release verification", path: "/api/v1/releases/verify/missing", statusCode: http.StatusOK},
	}

	var failures []string
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		var envelope struct {
			Data struct {
				Valid bool `json:"valid"`
			} `json:"data"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			failures = append(failures, fmt.Sprintf("%s 响应无法解析: %v", test.name, err))
			continue
		}
		if response.Code != test.statusCode {
			failures = append(failures, fmt.Sprintf("%s 状态码=%d，期望=%d", test.name, response.Code, test.statusCode))
			continue
		}
		if test.errorCode != "" && (envelope.Error == nil || envelope.Error.Code != test.errorCode) {
			failures = append(failures, fmt.Sprintf("%s 错误码=%+v，期望=%s", test.name, envelope.Error, test.errorCode))
		}
		if test.errorCode == "" && (envelope.Error != nil || envelope.Data.Valid) {
			failures = append(failures, fmt.Sprintf("%s 应返回 200 与 valid=false", test.name))
		}
	}
	if len(failures) > 0 {
		t.Fatalf("查询错误链分类失真: %s", strings.Join(failures, "; "))
	}
}
