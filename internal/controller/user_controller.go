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
// This can be seen as a Builder or Helper method to set up the controller
func (c *UserController) RegisterRoutes(r chi.Router) {
	r.Post("/auth/signup", c.HandleSignUp)
	r.Post("/auth/verify-aadhar/init", c.InitiateAadharVerification)
	r.Post("/auth/verify-aadhar/complete", c.CompleteAadharVerification)
}

type UserSignUpRequest struct {
	AuthUID   string `json:"auth_uid"` // Typically extracted from context/token
	Email     string `json:"email"`
	Name      string `json:"name"`
	PhnNumber string `json:"phn_number"`
}

type InitiateAadharRequest struct {
	UserID       uuid.UUID `json:"user_id"` // Authentication
	AadharNumber string    `json:"aadhar_number"`
}

type CompleteAadharRequest struct {
	UserID        uuid.UUID `json:"user_id"` // Authentication
	TransactionID string    `json:"transaction_id"`
	OTP           string    `json:"otp"`
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
		RespondError(w, http.StatusBadRequest, err.Error()) // Often a bad request (bad OTP)
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
