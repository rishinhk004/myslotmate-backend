package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type BookingController struct {
	bookingService service.BookingService
}

func NewBookingController(s service.BookingService) *BookingController {
	return &BookingController{bookingService: s}
}

func (c *BookingController) RegisterRoutes(r chi.Router) {
	r.Route("/bookings", func(r chi.Router) {
		r.Post("/", c.CreateBooking)
		r.Get("/user/{userID}", c.GetUserBookings)
	})
}

type CreateBookingRequest struct {
	UserID   uuid.UUID `json:"user_id"` // From Auth
	EventID  uuid.UUID `json:"event_id"`
	Quantity int       `json:"quantity"`
}

func (c *BookingController) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.BookingCreateRequest{
		EventID:  req.EventID,
		Quantity: req.Quantity,
	}

	booking, err := c.bookingService.CreateBooking(r.Context(), req.UserID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, booking)
}

func (c *BookingController) GetUserBookings(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	bookings, err := c.bookingService.GetUserBookings(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, bookings)
}
