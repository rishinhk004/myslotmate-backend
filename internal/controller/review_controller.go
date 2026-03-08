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
		r.Get("/event/{eventID}/rating", c.GetAverageRating)
		r.Get("/host/{hostID}", c.GetHostReviews)
	})
}

type CreateReviewRequestBody struct {
	UserID      uuid.UUID `json:"user_id"`
	EventID     uuid.UUID `json:"event_id"`
	Rating      int       `json:"rating"`
	Name        *string   `json:"name"`
	Description string    `json:"description"`
	PhotoURLs   []string  `json:"photo_urls,omitempty"`
}

func (c *ReviewController) CreateReview(w http.ResponseWriter, r *http.Request) {
	var req CreateReviewRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.ReviewCreateRequest{
		EventID:     req.EventID,
		Rating:      req.Rating,
		Name:        req.Name,
		Description: req.Description,
		PhotoURLs:   req.PhotoURLs,
	}

	review, err := c.reviewService.CreateReview(r.Context(), req.UserID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, review)
}

func (c *ReviewController) GetEventReviews(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
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

func (c *ReviewController) GetAverageRating(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	avg, count, err := c.reviewService.GetAverageRating(r.Context(), eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"average_rating": avg,
		"total_reviews":  count,
	})
}

func (c *ReviewController) GetHostReviews(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	reviews, err := c.reviewService.GetHostReviews(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, reviews)
}
