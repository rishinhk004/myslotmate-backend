package service

import (
	"context"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type ReviewService interface {
	CreateReview(ctx context.Context, userID uuid.UUID, req ReviewCreateRequest) (*models.Review, error)
	GetEventReviews(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error)
}

type ReviewCreateRequest struct {
	EventID     uuid.UUID
	Name        *string
	Description string
}

type reviewService struct {
	reviewRepo repository.ReviewRepository
	dispatcher *event.Dispatcher
}

func NewReviewService(rr repository.ReviewRepository, d *event.Dispatcher) ReviewService {
	return &reviewService{
		reviewRepo: rr,
		dispatcher: d,
	}
}

func (s *reviewService) CreateReview(ctx context.Context, userID uuid.UUID, req ReviewCreateRequest) (*models.Review, error) {
	// 1. Analyze Sentiment (Mock/Stub or External API)
	// In a real app, call AI service here.
	var sentimentScore float64 = 0.8 // Dummy positive score

	newReview := &models.Review{
		ID:             uuid.New(),
		EventID:        req.EventID,
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		Reply:          []string{},
		SentimentScore: &sentimentScore,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.reviewRepo.Create(ctx, newReview); err != nil {
		return nil, err
	}

	// 2. Publish Event (e.g., to trigger re-calculation of event rating)
	// s.dispatcher.Publish("review_created", newReview)

	return newReview, nil
}

func (s *reviewService) GetEventReviews(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error) {
	return s.reviewRepo.ListByEventID(ctx, eventID)
}
