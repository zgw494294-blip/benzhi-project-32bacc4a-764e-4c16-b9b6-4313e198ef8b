package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"trapreview/internal/domain"
)

const maxRequestBody = 1 << 20

type responseEnvelope struct {
	Data  any          `json:"data,omitempty"`
	Error *errorDetail `json:"error,omitempty"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{Data: value})
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	detail := &errorDetail{Code: "internal_error", Message: "服务内部错误"}
	var domainError *domain.Error
	var maxBytesError *http.MaxBytesError
	var decodeErr *jsonDecodeError
	switch {
	case errors.As(err, &domainError):
		detail = &errorDetail{Code: string(domainError.Code), Message: domainError.Message, Field: domainError.Field}
		switch domainError.Code {
		case domain.CodeNotFound:
			status = http.StatusNotFound
		case domain.CodeConflict, domain.CodeInvalidState, domain.CodeDuplicateRelease, domain.CodeUnresolved:
			status = http.StatusConflict
		default:
			status = http.StatusBadRequest
		}
	case errors.As(err, &maxBytesError):
		status, detail = http.StatusRequestEntityTooLarge, &errorDetail{Code: "body_too_large", Message: "请求体超过 1 MiB 限制"}
	case errors.Is(err, context.DeadlineExceeded):
		status, detail = http.StatusGatewayTimeout, &errorDetail{Code: "request_timeout", Message: "请求处理超时"}
	case errors.As(err, &decodeErr):
		status, detail = http.StatusBadRequest, &errorDetail{Code: "invalid_json", Message: decodeErr.Error()}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(responseEnvelope{Error: detail})
}

type jsonDecodeError struct{ err error }

func (e *jsonDecodeError) Error() string  { return e.err.Error() }
func (e *jsonDecodeError) Unwrap() error { return e.err }

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &jsonDecodeError{err: err}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return &jsonDecodeError{err: errors.New("请求体只能包含一个 JSON 对象")}
	}
	return nil
}

func idempotencyKey(r *http.Request) string { return r.Header.Get("Idempotency-Key") }
