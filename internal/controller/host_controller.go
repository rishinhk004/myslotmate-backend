package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type HostController struct {
	hostService service.HostService
}

func NewHostController(s service.HostService) *HostController {
	return &HostController{hostService: s}
}

func (c *HostController) RegisterRoutes(r chi.Router) {
	r.Route("/hosts", func(r chi.Router) {
		r.Post("/apply", c.SubmitApplication)
		r.Post("/apply/draft", c.SaveDraft)
		r.Get("/application-status", c.GetApplicationStatus)
		r.Get("/me", c.GetMyHostProfile)
		r.Put("/me", c.UpdateProfile)
		r.Get("/dashboard", c.GetDashboardOverview)
	})
}

// ── Request types ───────────────────────────────────────────────────────────

type HostApplicationRequestBody struct {
	UserID          uuid.UUID `json:"user_id"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	City            string    `json:"city"`
	PhnNumber       string    `json:"phn_number"`
	ExperienceDesc  *string   `json:"experience_desc,omitempty"`
	Moods           []string  `json:"moods"`
	Description     *string   `json:"description,omitempty"`
	PreferredDays   []string  `json:"preferred_days"`
	GroupSize       *int      `json:"group_size,omitempty"`
	GovernmentIDURL *string   `json:"government_id_url,omitempty"`
	AvatarURL       *string   `json:"avatar_url,omitempty"`
	Tagline         *string   `json:"tagline,omitempty"`
	Bio             *string   `json:"bio,omitempty"`
}

type HostProfileUpdateRequestBody struct {
	Tagline         *string  `json:"tagline,omitempty"`
	Bio             *string  `json:"bio,omitempty"`
	AvatarURL       *string  `json:"avatar_url,omitempty"`
	City            *string  `json:"city,omitempty"`
	ExpertiseTags   []string `json:"expertise_tags,omitempty"`
	SocialInstagram *string  `json:"social_instagram,omitempty"`
	SocialLinkedin  *string  `json:"social_linkedin,omitempty"`
	SocialWebsite   *string  `json:"social_website,omitempty"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (c *HostController) SubmitApplication(w http.ResponseWriter, r *http.Request) {
	var req HostApplicationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.HostApplicationRequest{
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		City:            req.City,
		ExperienceDesc:  req.ExperienceDesc,
		Moods:           req.Moods,
		Description:     req.Description,
		PreferredDays:   req.PreferredDays,
		GroupSize:       req.GroupSize,
		GovernmentIDURL: req.GovernmentIDURL,
	}

	host, err := c.hostService.SubmitApplication(r.Context(), req.UserID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, host)
}

func (c *HostController) SaveDraft(w http.ResponseWriter, r *http.Request) {
	var req HostApplicationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.HostApplicationRequest{
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		City:            req.City,
		ExperienceDesc:  req.ExperienceDesc,
		Moods:           req.Moods,
		Description:     req.Description,
		PreferredDays:   req.PreferredDays,
		GroupSize:       req.GroupSize,
		GovernmentIDURL: req.GovernmentIDURL,
	}

	host, err := c.hostService.SaveDraft(r.Context(), req.UserID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, host)
}

func (c *HostController) GetApplicationStatus(w http.ResponseWriter, r *http.Request) {
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

	status, err := c.hostService.GetApplicationStatus(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]interface{}{
		"status": status,
	})
}

func (c *HostController) GetMyHostProfile(w http.ResponseWriter, r *http.Request) {
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

	host, err := c.hostService.GetHostByUserID(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if host == nil {
		RespondError(w, http.StatusNotFound, "Host profile not found")
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}

func (c *HostController) UpdateProfile(w http.ResponseWriter, r *http.Request) {
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

	var req HostProfileUpdateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.HostProfileUpdateRequest{
		Tagline:         req.Tagline,
		Bio:             req.Bio,
		AvatarURL:       req.AvatarURL,
		ExpertiseTags:   req.ExpertiseTags,
		SocialInstagram: req.SocialInstagram,
		SocialLinkedin:  req.SocialLinkedin,
		SocialWebsite:   req.SocialWebsite,
	}

	host, err := c.hostService.UpdateProfile(r.Context(), userID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}

func (c *HostController) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
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

	overview, err := c.hostService.GetDashboardOverview(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, overview)
}
