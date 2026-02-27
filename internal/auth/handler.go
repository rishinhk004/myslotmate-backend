package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
)

type Handler struct {
	authClient *auth.Client
}

func NewHandler(authClient *auth.Client) *Handler {
	return &Handler{
		authClient: authClient,
	}
}

type verifyRequest struct {
	IDToken string `json:"idToken"`
}

type userResponse struct {
	UID         string                 `json:"uid"`
	Email       string                 `json:"email,omitempty"`
	DisplayName string                 `json:"displayName,omitempty"`
	PhotoURL    string                 `json:"photoUrl,omitempty"`
	ProviderID  string                 `json:"providerId,omitempty"`
	Claims      map[string]interface{} `json:"claims,omitempty"`
}

// VerifyIDToken verifies a Firebase ID token (e.g. from Google sign-in) and returns basic user info.
// Expected payload:
//   POST /auth/signUp
//   { "idToken": "<FIREBASE_ID_TOKEN_FROM_CLIENT>" }
func (h *Handler) VerifyIDToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.IDToken == "" {
		http.Error(w, "idToken is required", http.StatusBadRequest)
		return
	}

	token, err := h.authClient.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	user, err := h.authClient.GetUser(ctx, token.UID)
	if err != nil {
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	resp := userResponse{
		UID:         user.UID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		PhotoURL:    user.PhotoURL,
		Claims:      token.Claims,
	}

	for _, info := range user.ProviderUserInfo {
		// Prefer Google provider info if available.
		if info.ProviderID == "google.com" {
			resp.ProviderID = info.ProviderID
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

