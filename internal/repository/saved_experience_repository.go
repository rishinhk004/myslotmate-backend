package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

// SavedExperienceRepository provides saved/bookmarked experience data access.
type SavedExperienceRepository interface {
	Save(ctx context.Context, se *models.SavedExperience) error
	Remove(ctx context.Context, userID, eventID uuid.UUID) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.SavedExperience, error)
	Exists(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

type postgresSavedExperienceRepository struct {
	db *sql.DB
}

func NewSavedExperienceRepository(db *sql.DB) SavedExperienceRepository {
	return &postgresSavedExperienceRepository{db: db}
}

func (r *postgresSavedExperienceRepository) Save(ctx context.Context, se *models.SavedExperience) error {
	query := `
		INSERT INTO saved_experiences (id, user_id, event_id, saved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, event_id) DO NOTHING
	`
	if se.ID == uuid.Nil {
		se.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query, se.ID, se.UserID, se.EventID, se.SavedAt)
	return err
}

func (r *postgresSavedExperienceRepository) Remove(ctx context.Context, userID, eventID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM saved_experiences WHERE user_id = $1 AND event_id = $2`, userID, eventID)
	return err
}

func (r *postgresSavedExperienceRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.SavedExperience, error) {
	query := `SELECT id, user_id, event_id, saved_at FROM saved_experiences WHERE user_id = $1 ORDER BY saved_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var saved []*models.SavedExperience
	for rows.Next() {
		se := &models.SavedExperience{}
		if err := rows.Scan(&se.ID, &se.UserID, &se.EventID, &se.SavedAt); err != nil {
			return nil, err
		}
		saved = append(saved, se)
	}
	return saved, nil
}

func (r *postgresSavedExperienceRepository) Exists(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM saved_experiences WHERE user_id = $1 AND event_id = $2)`
	err := r.db.QueryRowContext(ctx, query, userID, eventID).Scan(&exists)
	return exists, err
}
