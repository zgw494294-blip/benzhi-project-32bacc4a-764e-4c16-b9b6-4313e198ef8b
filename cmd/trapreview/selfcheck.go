package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type checkEnvelope struct {
	Data  json.RawMessage                 `json:"data"`
	Error *struct{ Code, Message string } `json:"error"`
}
type checkMutation struct {
	SurveyID        string `json:"surveyId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	ResourceID      string `json:"resourceId"`
}
type checkPublish struct {
	ExpectedVersion int64 `json:"expectedVersion"`
	Release         struct {
		VerificationCode string `json:"verificationCode"`
	} `json:"release"`
}
type checkVerification struct {
	Valid bool `json:"valid"`
}

func runSelfcheck(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	create, err := checkCommand[checkMutation](ctx, client, http.MethodPost, baseURL+"/api/v1/surveys", "check-create", map[string]any{"title": "秦岭红外相机调查", "leadResearcher": "调查员甲", "speciesCatalog": []string{"金丝猴", "羚牛"}})
	if err != nil {
		return err
	}
	station, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/stations", baseURL, create.SurveyID), "check-station", map[string]any{"expectedVersion": create.ExpectedVersion, "stationCode": "QL-001", "locationDescription": "秦岭北坡林线", "deployedAt": "2026-05-01T00:00:00Z", "actor": "调查员甲"})
	if err != nil {
		return err
	}
	locked, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/lock", baseURL, create.SurveyID), "check-lock", map[string]any{"expectedVersion": station.ExpectedVersion, "actor": "调查员甲"})
	if err != nil {
		return err
	}
	observation, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations", baseURL, create.SurveyID), "check-observation", map[string]any{"expectedVersion": locked.ExpectedVersion, "stationId": station.ResourceID, "capturedAt": "2026-05-02T08:30:00Z", "mediaRef": "s3://field/ql-001/0001.jpg", "mediaChecksum": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "primaryLabel": "金丝猴", "secondaryLabel": "羚牛", "actor": "调查员甲"})
	if err != nil {
		return err
	}
	verified, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/verify", baseURL, create.SurveyID), "check-verify-1", map[string]any{"expectedVersion": observation.ExpectedVersion, "actor": "自动核验器"})
	if err != nil {
		return err
	}
	adjudicated, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/%s/adjudications", baseURL, create.SurveyID, observation.ResourceID), "check-adjudicate", map[string]any{"expectedVersion": verified.ExpectedVersion, "reason": "两份独立标注不一致", "decision": "require_correction", "correctionNote": "复核原始影像并统一标注", "adjudicator": "复核员乙"})
	if err != nil {
		return err
	}
	corrected, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/observations/%s/corrections", baseURL, create.SurveyID, observation.ResourceID), "check-correct", map[string]any{"expectedVersion": adjudicated.ExpectedVersion, "primaryLabel": "金丝猴", "secondaryLabel": "金丝猴", "actor": "调查员甲"})
	if err != nil {
		return err
	}
	reverified, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/verify", baseURL, create.SurveyID), "check-verify-2", map[string]any{"expectedVersion": corrected.ExpectedVersion, "actor": "自动核验器"})
	if err != nil {
		return err
	}
	reviewed, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/review", baseURL, create.SurveyID), "check-review", map[string]any{"expectedVersion": reverified.ExpectedVersion, "actor": "调查员甲"})
	if err != nil {
		return err
	}
	approved, err := checkCommand[checkMutation](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/approve", baseURL, create.SurveyID), "check-approve", map[string]any{"expectedVersion": reviewed.ExpectedVersion, "reviewer": "复核员乙"})
	if err != nil {
		return err
	}
	published, err := checkCommand[checkPublish](ctx, client, http.MethodPost, fmt.Sprintf("%s/api/v1/surveys/%s/releases", baseURL, create.SurveyID), "check-publish", map[string]any{"expectedVersion": approved.ExpectedVersion, "actor": "数据负责人丙"})
	if err != nil {
		return err
	}
	verification, err := checkCommand[checkVerification](ctx, client, http.MethodGet, fmt.Sprintf("%s/api/v1/releases/verify/%s", baseURL, published.Release.VerificationCode), "", nil)
	if err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("发布凭据核验结果无效")
	}
	return nil
}

func checkCommand[T any](ctx context.Context, client *http.Client, method, url, key string, body any) (T, error) {
	var zero T
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return zero, err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return zero, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := client.Do(request)
	if err != nil {
		return zero, fmt.Errorf("自检请求 %s: %w", url, err)
	}
	defer response.Body.Close()
	var envelope checkEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&envelope); err != nil {
		return zero, fmt.Errorf("解析自检响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			return zero, fmt.Errorf("自检请求失败 %s: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return zero, fmt.Errorf("自检请求返回 HTTP %d", response.StatusCode)
	}
	var value T
	if err := json.Unmarshal(envelope.Data, &value); err != nil {
		return zero, fmt.Errorf("解析自检业务结果: %w", err)
	}
	return value, nil
}
