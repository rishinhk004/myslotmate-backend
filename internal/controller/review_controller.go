package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ReviewController struct {
	reviewService service.ReviewService
}

func NewReviewController(s service.ReviewService) *ReviewController {
	return &ReviewController{reviewService: s}
}

func (c *ReviewController) RegisterRoutes(r chi.Router) {
	r.Route("/reviews", func(r chi.Router) {
		r.Post("/", c.CreateReview)
		r.Get("/event/{eventID}", c.GetEventReviews)
	})
}

type CreateReviewRequest struct {
	UserID      uuid.UUID `json:"user_id"`
	EventID     uuid.UUID `json:"event_id"`
	Name        *string   `json:"name"`
	Description string    `json:"description"`
}

func (c *ReviewController) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.ReviewCreateRequest{
		EventID:     req.EventID,
		Name:        req.Name,
		Description: req.Description,
	}

	review, err := c.reviewService.CreateReview(r.Context(), req.UserID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, review)
}

func (c *ReviewController) GetEventReviews(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	reviews, err := c.reviewService.GetEventReviews(r.Context(), eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, reviews)
}
