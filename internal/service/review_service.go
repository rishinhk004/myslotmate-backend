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
	dispatcher *event.Dispatcher
}

func NewReviewService(rr repository.ReviewRepository, er repository.EventRepository, d *event.Dispatcher) ReviewService {
	return &reviewService{
		reviewRepo: rr,
		eventRepo:  er,
		dispatcher: d,
	}
}

func (s *reviewService) CreateReview(ctx context.Context, userID uuid.UUID, req ReviewCreateRequest) (*models.Review, error) {
	// Validate rating
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
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
