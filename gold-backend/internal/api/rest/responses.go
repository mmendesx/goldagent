package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
)

const (
	defaultLimit = 100
	maxLimit     = 1000
)

// PaginationParams holds parsed pagination query parameters.
type PaginationParams struct {
	Limit  int
	Offset int
}

// ParsePagination reads `limit` and `offset` from URL query parameters.
// Defaults: limit=100, offset=0. Caps limit at 1000. Negative offset is clamped to 0.
func ParsePagination(r *http.Request) PaginationParams {
	limit := defaultLimit
	offset := 0

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if rawOffset := r.URL.Query().Get("offset"); rawOffset != "" {
		if parsed, err := strconv.Atoi(rawOffset); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	return PaginationParams{Limit: limit, Offset: offset}
}

// ErrorResponse is the JSON shape returned for all error responses.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// PaginatedResponse wraps list payloads with pagination metadata.
type PaginatedResponse struct {
	Items   interface{} `json:"items"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
	Count   int         `json:"count"`
	HasMore bool        `json:"hasMore"`
}

// WriteJSON encodes payload as JSON and writes it to the response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// At this point headers are already sent; log via stderr only.
		_ = err
	}
}

// WriteError writes a structured error response with the given HTTP status code.
func WriteError(w http.ResponseWriter, status int, message string, code string) {
	WriteJSON(w, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}
