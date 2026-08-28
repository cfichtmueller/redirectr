package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// writeErrorResponse writes a structured error response
func writeErrorResponse(w http.ResponseWriter, statusCode int, code, message, details string) {

	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Code:    code,
		Message: message,
		Details: details,
	}

	b, err := json.Marshal(response)
	if err != nil {
		slog.Error("failed to marshal error response", "error", err)
		w.WriteHeader(statusCode)
		fmt.Fprintf(w, "{\"error\": \"%s\"}\n", response.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(b)
}
