package respond

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// JSON writes a successful JSON response.
func JSON(w http.ResponseWriter, status int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(envelope{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error writes an error JSON response.
func Error(w http.ResponseWriter, status int, errCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(envelope{
		Success: false,
		Error:   errCode,
		Message: message,
	})
}
