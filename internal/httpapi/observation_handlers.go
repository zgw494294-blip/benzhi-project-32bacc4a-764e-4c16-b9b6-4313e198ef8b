package httpapi

import (
	"net/http"
	"strconv"

	"trapreview/internal/application"
	"trapreview/internal/domain"
)

func (a *API) SubmitObservationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitObservationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.SubmitObservation(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *API) SubmitObservationBatchHandler(w http.ResponseWriter, r *http.Request) {
	var command application.SubmitObservationBatchCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.SubmitObservationBatch(r.Context(), command)
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

func (a *API) VerifySurveyHandler(w http.ResponseWriter, r *http.Request) {
	var command application.VerifySurveyCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.VerifySurvey(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) VerifyObservationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.VerifySurveyCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	command.ObservationIDs = []string{r.PathValue("observationID")}
	result, err := a.service.VerifySurvey(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) VerificationResultsHandler(w http.ResponseWriter, r *http.Request) {
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
	result, err := a.service.VerificationResults(r.Context(), application.VerificationQuery{SurveyID: r.PathValue("surveyID"), StationID: r.URL.Query().Get("stationId"), Status: r.URL.Query().Get("status"), QualityFlag: r.URL.Query().Get("qualityFlag"), Page: page, PageSize: pageSize})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryInt(r *http.Request, name string, defaultValue int) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, domain.Invalid(name, "必须是整数")
	}
	return parsed, nil
}

func (a *API) PendingAdjudicationsHandler(w http.ResponseWriter, r *http.Request) {
	items, err := a.service.PendingAdjudications(r.Context(), r.PathValue("surveyID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "count": len(items)})
}

func (a *API) AdjudicateHandler(w http.ResponseWriter, r *http.Request) {
	var command application.AdjudicateCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.ObservationID, command.IdempotencyKey = r.PathValue("surveyID"), r.PathValue("observationID"), idempotencyKey(r)
	result, err := a.service.Adjudicate(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) CorrectObservationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CorrectObservationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.ObservationID, command.IdempotencyKey = r.PathValue("surveyID"), r.PathValue("observationID"), idempotencyKey(r)
	result, err := a.service.CorrectObservation(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
