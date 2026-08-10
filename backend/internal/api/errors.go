package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorResponse is the JSON envelope every error response uses, so API
// clients parse one shape regardless of endpoint or status code.
type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes are part of the API contract: clients branch on Code, not on
// the human-readable Message, which may change wording over time.
const (
	codeInvalidRequest = "invalid_request"
	codeNotFound       = "not_found"
	codeInternal       = "internal_error"
)

func writeError(w http.ResponseWriter, logger *slog.Logger, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorResponse{Error: errorDetail{Code: code, Message: message}}); err != nil {
		logger.Error("write error response", "error", err)
	}
}

func writeJSON(w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Error("write json response", "error", err)
	}
}
