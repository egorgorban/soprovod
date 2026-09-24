package httpapi

import (
	"encoding/json"
	"net/http"
)

// writeJSON encodes v as JSON to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorResponse is the JSON shape returned on error, per the API contract in
// PLAN.md: {"error": "..."}.
type errorResponse struct {
	Error string `json:"error"`
}

// writeError writes a JSON error body {"error": msg} with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
