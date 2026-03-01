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
}

type postgresReviewRepository struct {
	db *sql.DB
}

func NewReviewRepository(db *sql.DB) ReviewRepository {
	return &postgresReviewRepository{db: db}
}

func (r *postgresReviewRepository) Create(ctx context.Context, review *models.Review) error {
	query := `
		INSERT INTO reviews (id, event_id, user_id, name, description, reply, sentiment_score, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if review.ID == uuid.Nil {
		review.ID = uuid.New()
	}

	// Handle string array for reply (using pq.Array)
	_, err := r.db.ExecContext(ctx, query,
		review.ID, review.EventID, review.UserID, review.Name, review.Description, pq.Array(review.Reply), review.SentimentScore, review.CreatedAt, review.UpdatedAt,
	)
	return err
}

func (r *postgresReviewRepository) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.Review, error) {
	query := `SELECT id, event_id, user_id, name, description, reply, sentiment_score, created_at, updated_at FROM reviews WHERE event_id = $1`
	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*models.Review
	for rows.Next() {
		rev := &models.Review{}
		var reply []string

		// Use pq.Array to scan into []string
		// Note: Depending on driver (pgx vs lib/pq), scanning might differ.
		// Assuming pq.Array works with database/sql standard scan if driver supports it.
		// If using pgx/v5 stdlib, it often handles arrays natively or requires specific types.
		// Safe fallback: scan into []uint8 and parse or rely on driver.
		// For now using pq.Array as wrapper if we imported github.com/lib/pq.
		// If using pgx exclusively, we might need pgtype.
		if err := rows.Scan(
			&rev.ID, &rev.EventID, &rev.UserID, &rev.Name, &rev.Description, pq.Array(&reply), &rev.SentimentScore, &rev.CreatedAt, &rev.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rev.Reply = reply
		reviews = append(reviews, rev)
	}
	return reviews, nil
}
