package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"myslotmate-backend/internal/lib/storage"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SupportController struct {
	supportService service.SupportService
	uploadService  *storage.UploadService // nil when storage is not configured
}

func NewSupportController(s service.SupportService, us *storage.UploadService) *SupportController {
	return &SupportController{supportService: s, uploadService: us}
}

func (c *SupportController) RegisterRoutes(r chi.Router) {
	r.Route("/support", func(r chi.Router) {
		r.Post("/", c.CreateTicket)
		r.Get("/{ticketID}", c.GetTicket)
		r.Get("/user/{userID}", c.GetUserTickets)
		r.Post("/{ticketID}/message", c.AddMessage)
		r.Post("/{ticketID}/resolve", c.ResolveTicket)
	})
}

type AddMessageRequestBody struct {
	Message string `json:"message"`
}

// CreateTicket handles both JSON-only and multipart/form-data (with evidence files).
//
// Multipart fields:
//
//	user_id, category, subject, message, reported_user_id (optional),
//	event_id (optional), session_date (optional, RFC3339), report_reason (optional),
//	is_urgent ("true"/"false"), evidence (file field, multiple allowed)
func (c *SupportController) CreateTicket(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// ---------- JSON path (no files) ----------
	if contentType == "" || contentType == "application/json" || !isMultipart(contentType) {
		var req struct {
			UserID         uuid.UUID              `json:"user_id"`
			Category       models.SupportCategory `json:"category"`
			Subject        string                 `json:"subject"`
			Message        string                 `json:"message"`
			ReportedUserID *uuid.UUID             `json:"reported_user_id,omitempty"`
			EventID        *uuid.UUID             `json:"event_id,omitempty"`
			SessionDate    *time.Time             `json:"session_date,omitempty"`
			ReportReason   *models.ReportReason   `json:"report_reason,omitempty"`
			EvidenceURLs   []string               `json:"evidence_urls,omitempty"`
			IsUrgent       bool                   `json:"is_urgent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		svcReq := service.CreateSupportTicketRequest{
			Category:       req.Category,
			Subject:        req.Subject,
			Message:        req.Message,
			ReportedUserID: req.ReportedUserID,
			EventID:        req.EventID,
			SessionDate:    req.SessionDate,
			ReportReason:   req.ReportReason,
			EvidenceURLs:   req.EvidenceURLs,
			IsUrgent:       req.IsUrgent,
		}

		ticket, err := c.supportService.CreateTicket(r.Context(), req.UserID, svcReq)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		RespondSuccess(w, http.StatusCreated, ticket)
		return
	}

	// ---------- Multipart path (with file uploads) ----------
	const maxBody = 60 << 20 // 60 MB total body limit
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(maxBody); err != nil {
		RespondError(w, http.StatusBadRequest, "Request too large or invalid multipart form")
		return
	}

	userID, err := uuid.Parse(r.FormValue("user_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	category := models.SupportCategory(r.FormValue("category"))
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	var reportedUserID *uuid.UUID
	if v := r.FormValue("reported_user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			reportedUserID = &id
		}
	}

	var eventID *uuid.UUID
	if v := r.FormValue("event_id"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			eventID = &id
		}
	}

	var sessionDate *time.Time
	if v := r.FormValue("session_date"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			sessionDate = &t
		}
	}

	var reportReason *models.ReportReason
	if v := r.FormValue("report_reason"); v != "" {
		rr := models.ReportReason(v)
		reportReason = &rr
	}

	isUrgent := r.FormValue("is_urgent") == "true"

	// Upload evidence files to S3
	var evidenceURLs []string
	files := r.MultipartForm.File["evidence"]
	if len(files) > 0 {
		if c.uploadService == nil {
			RespondError(w, http.StatusServiceUnavailable, "File upload is not configured")
			return
		}
		results, err := c.uploadService.UploadMultiple(r.Context(), "support/evidence", files)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, res := range results {
			evidenceURLs = append(evidenceURLs, res.URL)
		}
	}

	svcReq := service.CreateSupportTicketRequest{
		Category:       category,
		Subject:        subject,
		Message:        message,
		ReportedUserID: reportedUserID,
		EventID:        eventID,
		SessionDate:    sessionDate,
		ReportReason:   reportReason,
		EvidenceURLs:   evidenceURLs,
		IsUrgent:       isUrgent,
	}

	ticket, err := c.supportService.CreateTicket(r.Context(), userID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(w, http.StatusCreated, ticket)
}

func (c *SupportController) GetTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	ticket, err := c.supportService.GetTicket(r.Context(), ticketID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, ticket)
}

func (c *SupportController) GetUserTickets(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	tickets, err := c.supportService.GetUserTickets(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, tickets)
}

func (c *SupportController) AddMessage(w http.ResponseWriter, r *http.Request) {
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req AddMessageRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ticket, err := c.supportService.AddMessage(r.Context(), ticketID, req.Message)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, ticket)
}

func (c *SupportController) ResolveTicket(w http.ResponseWriter, r *http.Request) {
	ticketID, err := uuid.Parse(chi.URLParam(r, "ticketID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	if err := c.supportService.ResolveTicket(r.Context(), ticketID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "Ticket resolved"})
}

// isMultipart returns true if the Content-Type header indicates multipart/form-data.
func isMultipart(ct string) bool {
	return len(ct) >= 19 && ct[:19] == "multipart/form-data"
}
