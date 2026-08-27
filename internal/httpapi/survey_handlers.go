package httpapi

import (
	"net/http"

	"trapreview/internal/application"
)

func (a *API) CreateSurveyHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CreateSurveyCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.IdempotencyKey = idempotencyKey(r)
	result, err := a.service.CreateSurvey(r.Context(), command)
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

func (a *API) GetSurveyHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.SurveyDetail(r.Context(), r.PathValue("surveyID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ReviseSurveyHandler(w http.ResponseWriter, r *http.Request) {
	var command application.ReviseSurveyCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.ReviseSurvey(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) AddStationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.AddStationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.AddStation(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) UpdateStationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.UpdateStationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.StationID, command.IdempotencyKey = r.PathValue("surveyID"), r.PathValue("stationID"), idempotencyKey(r)
	result, err := a.service.UpdateStation(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) RemoveStationHandler(w http.ResponseWriter, r *http.Request) {
	var command application.RemoveStationCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.StationID, command.IdempotencyKey = r.PathValue("surveyID"), r.PathValue("stationID"), idempotencyKey(r)
	result, err := a.service.RemoveStation(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) LockProtocolHandler(w http.ResponseWriter, r *http.Request) {
	var command application.LockProtocolCommand
	if err := decodeJSON(w, r, &command); err != nil {
		writeError(w, err)
		return
	}
	command.SurveyID, command.IdempotencyKey = r.PathValue("surveyID"), idempotencyKey(r)
	result, err := a.service.LockProtocol(r.Context(), command)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
