package controller

import (
	"encoding/json"
	"net/http"
)

// APIResponse standardizes the JSON response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// RespondJSON sends a JSON response
func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

// RespondError sends a standard error response
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, APIResponse{
		Success: false,
		Message: message,
		Error:   message, // Or a more technical error details
	})
}

// RespondSuccess sends a standard success response
func RespondSuccess(w http.ResponseWriter, status int, data interface{}) {
	RespondJSON(w, status, APIResponse{
		Success: true,
		Data:    data,
	})
}
