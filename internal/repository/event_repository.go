package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

type EventRepository interface {
	Create(ctx context.Context, event *models.Event) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error)
	ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error)
}

type postgresEventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) EventRepository {
	return &postgresEventRepository{db: db}
}

func (r *postgresEventRepository) Create(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (id, host_id, name, time, end_time, capacity, ai_suggestion, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.HostID, event.Name, event.Time, event.EndTime, event.Capacity, event.AISuggestion, event.CreatedAt, event.UpdatedAt,
	)
	return err
}

func (r *postgresEventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	event := &models.Event{}
	query := `SELECT id, host_id, name, time, end_time, capacity, ai_suggestion, created_at, updated_at FROM events WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID, &event.HostID, &event.Name, &event.Time, &event.EndTime, &event.Capacity, &event.AISuggestion, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return event, nil
}

func (r *postgresEventRepository) ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error) {
	query := `SELECT id, host_id, name, time, end_time, capacity, ai_suggestion, created_at, updated_at FROM events WHERE host_id = $1`
	rows, err := r.db.QueryContext(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event := &models.Event{}
		if err := rows.Scan(
			&event.ID, &event.HostID, &event.Name, &event.Time, &event.EndTime, &event.Capacity, &event.AISuggestion, &event.CreatedAt, &event.UpdatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}
