package service

import (
	"context"
	"errors"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type HostService interface {
	// Application flow (Become a Host)
	SubmitApplication(ctx context.Context, userID uuid.UUID, req HostApplicationRequest) (*models.Host, error)
	SaveDraft(ctx context.Context, userID uuid.UUID, req HostApplicationRequest) (*models.Host, error)
	GetApplicationStatus(ctx context.Context, userID uuid.UUID) (*models.Host, error)

	// Admin — approve / reject applications
	ApproveApplication(ctx context.Context, hostID uuid.UUID) (*models.Host, error)
	RejectApplication(ctx context.Context, hostID uuid.UUID, reason string) (*models.Host, error)
	ListPendingApplications(ctx context.Context) ([]*models.Host, error)

	// Profile management
	GetHostByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error)
	UpdateProfile(ctx context.Context, hostID uuid.UUID, req HostProfileUpdateRequest) (*models.Host, error)

	// Dashboard overview
	GetDashboardOverview(ctx context.Context, hostID uuid.UUID) (*HostDashboardOverview, error)
}

// HostApplicationRequest maps to the "Become a Host" form (Steps 1 & 2).
type HostApplicationRequest struct {
	FirstName       string   `json:"first_name"`
	LastName        string   `json:"last_name"`
	City            string   `json:"city"`
	ExperienceDesc  *string  `json:"experience_desc,omitempty"`
	Moods           []string `json:"moods,omitempty"`
	Description     *string  `json:"description,omitempty"`
	PreferredDays   []string `json:"preferred_days,omitempty"`
	GroupSize       *int     `json:"group_size,omitempty"`
	GovernmentIDURL *string  `json:"government_id_url,omitempty"`
}

// HostProfileUpdateRequest maps to the Host Profile edit screen.
type HostProfileUpdateRequest struct {
	FirstName       *string  `json:"first_name,omitempty"`
	LastName        *string  `json:"last_name,omitempty"`
	AvatarURL       *string  `json:"avatar_url,omitempty"`
	Tagline         *string  `json:"tagline,omitempty"`
	Bio             *string  `json:"bio,omitempty"`
	ExpertiseTags   []string `json:"expertise_tags,omitempty"`
	SocialInstagram *string  `json:"social_instagram,omitempty"`
	SocialLinkedin  *string  `json:"social_linkedin,omitempty"`
	SocialWebsite   *string  `json:"social_website,omitempty"`
}

// HostDashboardOverview powers the Host Dashboard overview screen.
type HostDashboardOverview struct {
	UpcomingToday   int     `json:"upcoming_today"`
	MonthlyBookings int     `json:"monthly_bookings"`
	TotalEarnings   int64   `json:"total_earnings_cents"`
	AvgRating       float64 `json:"avg_rating"`
	TotalReviews    int     `json:"total_reviews"`
}

type hostService struct {
	hostRepo    repository.HostRepository
	userRepo    repository.UserRepository
	eventRepo   repository.EventRepository
	bookingRepo repository.BookingRepository
	payoutRepo  repository.PayoutRepository
	dispatcher  *event.Dispatcher
}

func NewHostService(
	hr repository.HostRepository,
	ur repository.UserRepository,
	er repository.EventRepository,
	br repository.BookingRepository,
	pr repository.PayoutRepository,
	d *event.Dispatcher,
) HostService {
	return &hostService{
		hostRepo:    hr,
		userRepo:    ur,
		eventRepo:   er,
		bookingRepo: br,
		payoutRepo:  pr,
		dispatcher:  d,
	}
}

func (s *hostService) SaveDraft(ctx context.Context, userID uuid.UUID, req HostApplicationRequest) (*models.Host, error) {
	return s.saveHostApplication(ctx, userID, req, models.HostApplicationDraft)
}

func (s *hostService) SubmitApplication(ctx context.Context, userID uuid.UUID, req HostApplicationRequest) (*models.Host, error) {
	return s.saveHostApplication(ctx, userID, req, models.HostApplicationPending)
}

func (s *hostService) saveHostApplication(ctx context.Context, userID uuid.UUID, req HostApplicationRequest, status models.HostApplicationStatus) (*models.Host, error) {
	// 1. Check if user exists and is verified
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if !user.IsVerified {
		return nil, errors.New("user is not verified")
	}

	now := time.Now()

	// 2. Check if host application already exists — update if so
	existing, err := s.hostRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		// Can only update if draft or rejected
		if existing.ApplicationStatus != models.HostApplicationDraft && existing.ApplicationStatus != models.HostApplicationRejected {
			return nil, errors.New("application already submitted")
		}
		existing.FirstName = req.FirstName
		existing.LastName = req.LastName
		existing.City = req.City
		existing.ExperienceDesc = req.ExperienceDesc
		existing.Moods = pq.StringArray(req.Moods)
		existing.Description = req.Description
		existing.PreferredDays = pq.StringArray(req.PreferredDays)
		existing.GroupSize = req.GroupSize
		existing.GovernmentIDURL = req.GovernmentIDURL
		existing.ApplicationStatus = status
		if status == models.HostApplicationPending {
			existing.SubmittedAt = &now
		}
		if err := s.hostRepo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// 3. Create new host application
	newHost := &models.Host{
		ID:                uuid.New(),
		UserID:            userID,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		PhnNumber:         user.PhnNumber,
		City:              req.City,
		ApplicationStatus: status,
		ExperienceDesc:    req.ExperienceDesc,
		Moods:             pq.StringArray(req.Moods),
		Description:       req.Description,
		PreferredDays:     pq.StringArray(req.PreferredDays),
		GroupSize:         req.GroupSize,
		GovernmentIDURL:   req.GovernmentIDURL,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if status == models.HostApplicationPending {
		newHost.SubmittedAt = &now
	}

	if err := s.hostRepo.Create(ctx, newHost); err != nil {
		return nil, err
	}

	if status == models.HostApplicationPending {
		s.dispatcher.Publish(event.HostCreated, newHost)
	}

	return newHost, nil
}

func (s *hostService) GetApplicationStatus(ctx context.Context, userID uuid.UUID) (*models.Host, error) {
	return s.hostRepo.GetByUserID(ctx, userID)
}

func (s *hostService) GetHostByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error) {
	return s.hostRepo.GetByUserID(ctx, userID)
}

func (s *hostService) UpdateProfile(ctx context.Context, hostID uuid.UUID, req HostProfileUpdateRequest) (*models.Host, error) {
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, errors.New("host not found")
	}

	if req.FirstName != nil {
		host.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		host.LastName = *req.LastName
	}
	if req.AvatarURL != nil {
		host.AvatarURL = req.AvatarURL
	}
	if req.Tagline != nil {
		host.Tagline = req.Tagline
	}
	if req.Bio != nil {
		host.Bio = req.Bio
	}
	if req.ExpertiseTags != nil {
		host.ExpertiseTags = pq.StringArray(req.ExpertiseTags)
	}
	if req.SocialInstagram != nil {
		host.SocialInstagram = req.SocialInstagram
	}
	if req.SocialLinkedin != nil {
		host.SocialLinkedin = req.SocialLinkedin
	}
	if req.SocialWebsite != nil {
		host.SocialWebsite = req.SocialWebsite
	}

	if err := s.hostRepo.Update(ctx, host); err != nil {
		return nil, err
	}

	return host, nil
}

func (s *hostService) GetDashboardOverview(ctx context.Context, hostID uuid.UUID) (*HostDashboardOverview, error) {
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, errors.New("host not found")
	}

	// Earnings
	earnings, err := s.payoutRepo.GetHostEarnings(ctx, hostID)
	if err != nil {
		return nil, err
	}

	overview := &HostDashboardOverview{
		TotalReviews: host.TotalReviews,
	}
	if host.AvgRating != nil {
		overview.AvgRating = *host.AvgRating
	}
	if earnings != nil {
		overview.TotalEarnings = earnings.TotalEarningsCents
	}

	return overview, nil
}

// ── Admin: Approve / Reject ─────────────────────────────────────────────────

func (s *hostService) ApproveApplication(ctx context.Context, hostID uuid.UUID) (*models.Host, error) {
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, errors.New("host not found")
	}

	if host.ApplicationStatus != models.HostApplicationPending && host.ApplicationStatus != models.HostApplicationUnderReview {
		return nil, errors.New("application is not in a reviewable state")
	}

	now := time.Now()
	host.ApplicationStatus = models.HostApplicationApproved
	host.ApprovedAt = &now

	if err := s.hostRepo.Update(ctx, host); err != nil {
		return nil, err
	}

	s.dispatcher.Publish(event.HostApproved, host)
	return host, nil
}

func (s *hostService) RejectApplication(ctx context.Context, hostID uuid.UUID, reason string) (*models.Host, error) {
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, errors.New("host not found")
	}

	if host.ApplicationStatus != models.HostApplicationPending && host.ApplicationStatus != models.HostApplicationUnderReview {
		return nil, errors.New("application is not in a reviewable state")
	}

	now := time.Now()
	host.ApplicationStatus = models.HostApplicationRejected
	host.RejectedAt = &now

	if err := s.hostRepo.Update(ctx, host); err != nil {
		return nil, err
	}

	s.dispatcher.Publish(event.HostRejected, host)
	return host, nil
}

func (s *hostService) ListPendingApplications(ctx context.Context) ([]*models.Host, error) {
	return s.hostRepo.ListByStatus(ctx, models.HostApplicationPending)
}
