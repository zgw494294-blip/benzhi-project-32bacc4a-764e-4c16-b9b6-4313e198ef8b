package httpapi

import (
	"net/http"
	"time"

	"trapreview/internal/application"
)

type API struct{ service *application.Service }

func NewHandler(service *application.Service) http.Handler {
	api := &API{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/surveys", api.CreateSurveyHandler)
	mux.HandleFunc("GET /api/v1/surveys/{surveyID}", api.GetSurveyHandler)
	mux.HandleFunc("PATCH /api/v1/surveys/{surveyID}", api.ReviseSurveyHandler)
	mux.HandleFunc("PUT /api/v1/surveys/{surveyID}", api.ReviseSurveyHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/stations", api.AddStationHandler)
	mux.HandleFunc("PUT /api/v1/surveys/{surveyID}/stations/{stationID}", api.UpdateStationHandler)
	mux.HandleFunc("PATCH /api/v1/surveys/{surveyID}/stations/{stationID}", api.UpdateStationHandler)
	mux.HandleFunc("DELETE /api/v1/surveys/{surveyID}/stations/{stationID}", api.RemoveStationHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/lock", api.LockProtocolHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/observations", api.SubmitObservationHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/observations/batch", api.SubmitObservationBatchHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/verify", api.VerifySurveyHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/observations/{observationID}/verify", api.VerifyObservationHandler)
	mux.HandleFunc("GET /api/v1/surveys/{surveyID}/verification-results", api.VerificationResultsHandler)
	mux.HandleFunc("GET /api/v1/surveys/{surveyID}/adjudications/pending", api.PendingAdjudicationsHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/observations/{observationID}/adjudications", api.AdjudicateHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/observations/{observationID}/corrections", api.CorrectObservationHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/review", api.RequestReviewHandler)
	mux.HandleFunc("GET /api/v1/surveys/{surveyID}/review-summary", api.ReviewSummaryHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/approve", api.ApproveHandler)
	mux.HandleFunc("POST /api/v1/surveys/{surveyID}/releases", api.PublishHandler)
	mux.HandleFunc("GET /api/v1/releases/verify/{verificationCode}", api.VerifyReleaseHandler)
	mux.HandleFunc("GET /api/v1/releases/verify/{verificationCode}/contents", api.ReleaseContentsHandler)
	return requestBoundary(mux)
}

func requestBoundary(next http.Handler) http.Handler {
	timeout := http.TimeoutHandler(next, 10*time.Second, `{"error":{"code":"request_timeout","message":"请求处理超时"}}`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		timeout.ServeHTTP(w, r)
	})
}
