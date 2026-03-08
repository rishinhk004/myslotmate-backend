package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// UserController handles HTTP requests for user operations
type UserController struct {
	userService service.UserService
}

// NewUserController Factory for UserController
func NewUserController(s service.UserService) *UserController {
	return &UserController{
		userService: s,
	}
}

// RegisterRoutes registers routes for the user controller on the provided router
func (c *UserController) RegisterRoutes(r chi.Router) {
	r.Post("/auth/signup", c.HandleSignUp)
	r.Post("/auth/verify-aadhar/init", c.InitiateAadharVerification)
	r.Post("/auth/verify-aadhar/complete", c.CompleteAadharVerification)
	r.Route("/users", func(r chi.Router) {
		r.Get("/me", c.GetProfile)
		r.Put("/me", c.UpdateProfile)
		r.Post("/saved-experiences", c.SaveExperience)
		r.Delete("/saved-experiences/{eventID}", c.UnsaveExperience)
		r.Get("/saved-experiences", c.GetSavedExperiences)
		r.Get("/saved-experiences/{eventID}/check", c.IsExperienceSaved)
	})
}

type UserSignUpRequest struct {
	AuthUID   string `json:"auth_uid"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	PhnNumber string `json:"phn_number"`
}

type InitiateAadharRequest struct {
	UserID       uuid.UUID `json:"user_id"`
	AadharNumber string    `json:"aadhar_number"`
}

type CompleteAadharRequest struct {
	UserID        uuid.UUID `json:"user_id"`
	TransactionID string    `json:"transaction_id"`
	OTP           string    `json:"otp"`
}

type UserProfileUpdateRequestBody struct {
	Name      *string `json:"name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	City      *string `json:"city,omitempty"`
}

type SaveExperienceRequestBody struct {
	UserID  uuid.UUID `json:"user_id"`
	EventID uuid.UUID `json:"event_id"`
}

func (c *UserController) InitiateAadharVerification(w http.ResponseWriter, r *http.Request) {
	var req InitiateAadharRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	txnID, err := c.userService.InitiateAadharVerification(r.Context(), req.UserID, req.AadharNumber)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{
		"transaction_id": txnID,
		"message":        "OTP sent successfully",
	})
}

func (c *UserController) CompleteAadharVerification(w http.ResponseWriter, r *http.Request) {
	var req CompleteAadharRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	err := c.userService.CompleteAadharVerification(r.Context(), req.UserID, req.TransactionID, req.OTP)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{
		"message": "User verified successfully",
	})
}

// HandleSignUp handles the POST /auth/signup endpoint
func (c *UserController) HandleSignUp(w http.ResponseWriter, r *http.Request) {
	var req UserSignUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.SignUpRequest{
		AuthUID:   req.AuthUID,
		Email:     req.Email,
		Name:      req.Name,
		PhnNumber: req.PhnNumber,
	}

	user, err := c.userService.SignUp(r.Context(), svcReq)
	if err != nil {
		if err.Error() == "user already exists" {
			RespondError(w, http.StatusConflict, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	RespondSuccess(w, http.StatusCreated, user)
}

func (c *UserController) GetProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	user, err := c.userService.GetProfile(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, user)
}

func (c *UserController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	var req UserProfileUpdateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.UserProfileUpdateRequest{
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
		City:      req.City,
	}

	user, err := c.userService.UpdateProfile(r.Context(), userID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, user)
}

func (c *UserController) SaveExperience(w http.ResponseWriter, r *http.Request) {
	var req SaveExperienceRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := c.userService.SaveExperience(r.Context(), req.UserID, req.EventID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, map[string]string{"message": "Experience saved"})
}

func (c *UserController) UnsaveExperience(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	if err := c.userService.UnsaveExperience(r.Context(), userID, eventID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "Experience unsaved"})
}

func (c *UserController) GetSavedExperiences(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	saved, err := c.userService.GetSavedExperiences(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, saved)
}

func (c *UserController) IsExperienceSaved(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	exists, err := c.userService.IsExperienceSaved(r.Context(), userID, eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]bool{"saved": exists})
}
