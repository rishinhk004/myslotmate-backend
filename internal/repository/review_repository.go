package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *models.Review) error
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error)
	GetAverageRating(ctx context.Context, eventID uuid.UUID) (float64, int, error)
	ListByEventIDs(ctx context.Context, eventIDs []uuid.UUID) ([]*models.Review, error)
	CountPendingReviewsByEventIDs(ctx context.Context, eventIDs []uuid.UUID, confirmedBookingCount int) (int, error)
}

type postgresReviewRepository struct {
	db *sql.DB
}

func NewReviewRepository(db *sql.DB) ReviewRepository {
	return &postgresReviewRepository{db: db}
}

func (r *postgresReviewRepository) Create(ctx context.Context, review *models.Review) error {
	query := `
		INSERT INTO reviews (id, event_id, user_id, rating, name, description, photo_urls, reply, sentiment_score, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		review.ID, review.EventID, review.UserID, review.Rating, review.Name, review.Description,
		pq.Array(review.PhotoURLs), pq.Array(review.Reply), review.SentimentScore,
		review.CreatedAt, review.UpdatedAt,
	)
	return err
}

func (r *postgresReviewRepository) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error) {
	query := `SELECT id, event_id, user_id, rating, name, description, photo_urls, reply, sentiment_score, created_at, updated_at FROM reviews WHERE event_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*models.Review
	for rows.Next() {
		rev := &models.Review{}
		var reply []string
		if err := rows.Scan(
			&rev.ID, &rev.EventID, &rev.UserID, &rev.Rating, &rev.Name, &rev.Description,
			pq.Array(&rev.PhotoURLs), pq.Array(&reply), &rev.SentimentScore,
			&rev.CreatedAt, &rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rev.Reply = reply
		reviews = append(reviews, rev)
	}
	return reviews, nil
}

// GetAverageRating returns the average star rating and total count for an event.
func (r *postgresReviewRepository) GetAverageRating(ctx context.Context, eventID uuid.UUID) (float64, int, error) {
	var avg sql.NullFloat64
	var count int
	query := `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM reviews WHERE event_id = $1`
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&avg, &count)
	if err != nil {
		return 0, 0, err
	}
	if !avg.Valid {
		return 0, 0, nil
	}
	return avg.Float64, count, nil
}

func (r *postgresReviewRepository) ListByEventIDs(ctx context.Context, eventIDs []uuid.UUID) ([]*models.Review, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, event_id, user_id, rating, name, description, photo_urls, reply, sentiment_score, created_at, updated_at FROM reviews WHERE event_id = ANY($1) ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(eventIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*models.Review
	for rows.Next() {
		rev := &models.Review{}
		var reply []string
		if err := rows.Scan(
			&rev.ID, &rev.EventID, &rev.UserID, &rev.Rating, &rev.Name, &rev.Description,
			pq.Array(&rev.PhotoURLs), pq.Array(&reply), &rev.SentimentScore,
			&rev.CreatedAt, &rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rev.Reply = reply
		reviews = append(reviews, rev)
	}
	return reviews, rows.Err()
}

func (r *postgresReviewRepository) CountPendingReviewsByEventIDs(ctx context.Context, eventIDs []uuid.UUID, confirmedBookingCount int) (int, error) {
	if len(eventIDs) == 0 {
		return 0, nil
	}
	// Count reviews that exist for these events
	var reviewCount int
	query := `SELECT COUNT(*) FROM reviews WHERE event_id = ANY($1)`
	err := r.db.QueryRowContext(ctx, query, pq.Array(eventIDs)).Scan(&reviewCount)
	if err != nil {
		return 0, err
	}
	// Pending reviews = confirmed bookings - existing reviews (floored at 0)
	pending := confirmedBookingCount - reviewCount
	if pending < 0 {
		pending = 0
	}
	return pending, nil
}
