package httpapi

import (
	"net/http"

	"trapreview/internal/application"
)

func (a *API) RequestReviewHandler(w http.ResponseWriter, r *http.Request) {
	var command application.RequestReviewCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.RequestReview(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ReviewSummaryHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.ReviewSummary(r.Context(), r.PathValue("surveyID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ApproveHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ApproveCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.Approve(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) PublishHandler(w http.ResponseWriter, r *http.Request) {
	var command application.PublishCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.Publish(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (a *API) VerifyReleaseHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.VerifyRelease(r.Context(), r.PathValue("verificationCode"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ReleaseContentsHandler(w http.ResponseWriter, r *http.Request) {
	page, err := queryInt(r, "page", 1)
	if err != nil {
		writeError(w, err)
		return
	}
	pageSize, err := queryInt(r, "pageSize", 50)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.ReleaseContents(r.Context(), application.ReleaseContentQuery{VerificationCode: r.PathValue("verificationCode"), Species: r.URL.Query().Get("species"), Page: page, PageSize: pageSize})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
