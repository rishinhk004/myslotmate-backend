package service

import (
	"context"
	"errors"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type ReviewService interface {
	CreateReview(ctx context.Context, userID uuid.UUID, req ReviewCreateRequest) (*models.Review, error)
	GetEventReviews(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error)
	GetAverageRating(ctx context.Context, eventID uuid.UUID) (float64, int, error)
	GetHostReviews(ctx context.Context, hostID uuid.UUID) ([]*models.Review, error)
	AddReply(ctx context.Context, reviewID uuid.UUID, reply string) error
}

type ReviewCreateRequest struct {
	EventID     uuid.UUID
	Rating      int
	Name        *string
	Description string
	PhotoURLs   []string
}

type reviewService struct {
	reviewRepo repository.ReviewRepository
	eventRepo  repository.EventRepository
	hostRepo   repository.HostRepository
	dispatcher *event.Dispatcher
}

func NewReviewService(rr repository.ReviewRepository, er repository.EventRepository, hr repository.HostRepository, d *event.Dispatcher) ReviewService {
	return &reviewService{
		reviewRepo: rr,
		eventRepo:  er,
		hostRepo:   hr,
		dispatcher: d,
	}
}

func (s *reviewService) CreateReview(ctx context.Context, userID uuid.UUID, req ReviewCreateRequest) (*models.Review, error) {
	// Validate rating
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}

	// Get event to find host_id
	evt, err := s.eventRepo.GetByID(ctx, req.EventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}

	// Analyze Sentiment (Mock/Stub or External API)
	var sentimentScore float64 = 0.8

	newReview := &models.Review{
		ID:             uuid.New(),
		EventID:        req.EventID,
		UserID:         userID,
		Rating:         req.Rating,
		Name:           req.Name,
		Description:    req.Description,
		PhotoURLs:      req.PhotoURLs,
		Reply:          []string{},
		SentimentScore: &sentimentScore,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.reviewRepo.Create(ctx, newReview); err != nil {
		return nil, err
	}

	// Increment event's review count
	if err := s.eventRepo.IncrementReviewCount(ctx, req.EventID); err != nil {
		// Log but don't fail - counter reconciliation can happen in background
	}

	// Update event's average rating from reviews for this event
	eventReviews, err := s.reviewRepo.ListByEventID(ctx, req.EventID)
	if err == nil && len(eventReviews) > 0 {
		eventTotalRating := 0
		for _, r := range eventReviews {
			eventTotalRating += r.Rating
		}
		eventAvgRating := float64(eventTotalRating) / float64(len(eventReviews))
		if err := s.eventRepo.UpdateAverageRating(ctx, req.EventID, eventAvgRating); err != nil {
			// Log but don't fail
		}
	}

	// Update host's average rating
	// Get all event IDs for this host
	eventIDs, err := s.eventRepo.ListByHostIDForIDs(ctx, evt.HostID)
	if err != nil {
		// Log but don't fail the review creation
		return newReview, nil
	}

	// Get all reviews for these events and calculate average
	if len(eventIDs) > 0 {
		reviews, err := s.reviewRepo.ListByEventIDs(ctx, eventIDs)
		if err != nil {
			// Log but don't fail the review creation
			return newReview, nil
		}

		if len(reviews) > 0 {
			totalRating := 0
			for _, r := range reviews {
				totalRating += r.Rating
			}
			avgRating := float64(totalRating) / float64(len(reviews))
			if err := s.hostRepo.UpdateAverageRating(ctx, evt.HostID, avgRating, len(reviews)); err != nil {
				// Log but don't fail the review creation
				return newReview, nil
			}
		}
	}

	return newReview, nil
}

func (s *reviewService) GetEventReviews(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error) {
	return s.reviewRepo.ListByEventID(ctx, eventID)
}

func (s *reviewService) GetAverageRating(ctx context.Context, eventID uuid.UUID) (float64, int, error) {
	return s.reviewRepo.GetAverageRating(ctx, eventID)
}

func (s *reviewService) GetHostReviews(ctx context.Context, hostID uuid.UUID) ([]*models.Review, error) {
	// Get all event IDs for this host
	eventIDs, err := s.eventRepo.ListByHostIDForIDs(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if len(eventIDs) == 0 {
		return []*models.Review{}, nil
	}
	return s.reviewRepo.ListByEventIDs(ctx, eventIDs)
}

func (s *reviewService) AddReply(ctx context.Context, reviewID uuid.UUID, reply string) error {
	if reply == "" {
		return errors.New("reply cannot be empty")
	}
	return s.reviewRepo.AddReply(ctx, reviewID, reply)
}
