package repository

import (
	"context"
	"database/sql"
	"fmt"
	"myslotmate-backend/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type EventRepository interface {
	Create(ctx context.Context, event *models.Event) error
	Update(ctx context.Context, event *models.Event) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error)
	ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error)
	ListByHostIDFiltered(ctx context.Context, hostID uuid.UUID, status *models.EventStatus, search string, sortBy string, limit, offset int) ([]*models.Event, error)
	ListByDateRange(ctx context.Context, hostID uuid.UUID, start, end time.Time) ([]*models.Event, error)
	ListTodayByHostID(ctx context.Context, hostID uuid.UUID, dayStart, dayEnd time.Time) ([]*models.Event, error)
	ListByHostIDForIDs(ctx context.Context, hostID uuid.UUID) ([]uuid.UUID, error)
	ListPublished(ctx context.Context, limit, offset int) ([]*models.Event, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.EventStatus) error
}

type postgresEventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) EventRepository {
	return &postgresEventRepository{db: db}
}

var eventColumns = `id, host_id,
	title, hook_line, mood, description,
	cover_image_url, gallery_urls,
	is_online, location, location_lat, location_lng, duration_minutes, min_group_size, max_group_size, capacity,
	price_cents, is_free, time, end_time, is_recurring, recurrence_rule,
	cancellation_policy, status, published_at, paused_at,
	ai_suggestion, avg_rating, total_bookings,
	created_at, updated_at`

func scanEvent(row interface {
	Scan(dest ...interface{}) error
}) (*models.Event, error) {
	e := &models.Event{}
	err := row.Scan(
		&e.ID, &e.HostID,
		&e.Title, &e.HookLine, &e.Mood, &e.Description,
		&e.CoverImageURL, &e.GalleryURLs,
		&e.IsOnline, &e.Location, &e.LocationLat, &e.LocationLng, &e.DurationMinutes, &e.MinGroupSize, &e.MaxGroupSize, &e.Capacity,
		&e.PriceCents, &e.IsFree, &e.Time, &e.EndTime, &e.IsRecurring, &e.RecurrenceRule,
		&e.CancellationPolicy, &e.Status, &e.PublishedAt, &e.PausedAt,
		&e.AISuggestion, &e.AvgRating, &e.TotalBookings,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	normalizedMood, err := models.NormalizeEventMood(e.Mood)
	if err != nil {
		return nil, err
	}
	e.Mood = normalizedMood
	return e, nil
}

func (r *postgresEventRepository) Create(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (
			id, host_id,
			title, hook_line, mood, description,
			cover_image_url, gallery_urls,
			is_online, location, location_lat, location_lng, duration_minutes, min_group_size, max_group_size, capacity,
			price_cents, is_free, time, end_time, is_recurring, recurrence_rule,
			cancellation_policy, status, published_at,
			ai_suggestion,
			created_at, updated_at
		) VALUES (
			$1, $2,
			$3, $4, $5, $6,
			$7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22,
			$23, $24, $25,
			$26,
			$27, $28
		)
	`
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		event.ID, event.HostID,
		event.Title, event.HookLine, event.Mood, event.Description,
		event.CoverImageURL, pq.Array(event.GalleryURLs),
		event.IsOnline, event.Location, event.LocationLat, event.LocationLng, event.DurationMinutes, event.MinGroupSize, event.MaxGroupSize, event.Capacity,
		event.PriceCents, event.IsFree, event.Time, event.EndTime, event.IsRecurring, event.RecurrenceRule,
		event.CancellationPolicy, event.Status, event.PublishedAt,
		event.AISuggestion,
		event.CreatedAt, event.UpdatedAt,
	)
	return err
}

func (r *postgresEventRepository) Update(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events SET
			title = $1, hook_line = $2, mood = $3, description = $4,
			cover_image_url = $5, gallery_urls = $6,
			is_online = $7, location = $8, location_lat = $9, location_lng = $10, duration_minutes = $11, min_group_size = $12, max_group_size = $13, capacity = $14,
			price_cents = $15, is_free = $16, time = $17, end_time = $18, is_recurring = $19, recurrence_rule = $20,
			cancellation_policy = $21, status = $22, published_at = $23, paused_at = $24,
			ai_suggestion = $25, avg_rating = $26, total_bookings = $27
		WHERE id = $28
	`
	_, err := r.db.ExecContext(ctx, query,
		event.Title, event.HookLine, event.Mood, event.Description,
		event.CoverImageURL, pq.Array(event.GalleryURLs),
		event.IsOnline, event.Location, event.LocationLat, event.LocationLng, event.DurationMinutes, event.MinGroupSize, event.MaxGroupSize, event.Capacity,
		event.PriceCents, event.IsFree, event.Time, event.EndTime, event.IsRecurring, event.RecurrenceRule,
		event.CancellationPolicy, event.Status, event.PublishedAt, event.PausedAt,
		event.AISuggestion, event.AvgRating, event.TotalBookings,
		event.ID,
	)
	return err
}

func (r *postgresEventRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events WHERE id = $1`
	return scanEvent(r.db.QueryRowContext(ctx, query, id))
}

func (r *postgresEventRepository) ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events WHERE host_id = $1 ORDER BY created_at DESC`
	return r.scanEvents(ctx, query, hostID)
}

func (r *postgresEventRepository) ListByHostIDFiltered(ctx context.Context, hostID uuid.UUID, status *models.EventStatus, search string, sortBy string, limit, offset int) ([]*models.Event, error) {
	args := []interface{}{hostID}
	conditions := []string{"host_id = $1"}
	idx := 2

	if status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, *status)
		idx++
	}
	if search != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", idx))
		args = append(args, "%"+search+"%")
		idx++
	}

	orderBy := "created_at DESC"
	if sortBy == "oldest" {
		orderBy = "created_at ASC"
	}

	query := fmt.Sprintf(`SELECT %s FROM events WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		eventColumns, strings.Join(conditions, " AND "), orderBy, idx, idx+1)
	args = append(args, limit, offset)

	return r.scanEvents(ctx, query, args...)
}

func (r *postgresEventRepository) ListByDateRange(ctx context.Context, hostID uuid.UUID, start, end time.Time) ([]*models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events WHERE host_id = $1 AND time >= $2 AND time < $3 ORDER BY time ASC`
	return r.scanEvents(ctx, query, hostID, start, end)
}

func (r *postgresEventRepository) ListTodayByHostID(ctx context.Context, hostID uuid.UUID, dayStart, dayEnd time.Time) ([]*models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events WHERE host_id = $1 AND time >= $2 AND time < $3 AND status = 'live' ORDER BY time ASC`
	return r.scanEvents(ctx, query, hostID, dayStart, dayEnd)
}

func (r *postgresEventRepository) ListByHostIDForIDs(ctx context.Context, hostID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT id FROM events WHERE host_id = $1`
	rows, err := r.db.QueryContext(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *postgresEventRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.EventStatus) error {
	var query string
	switch status {
	case models.EventStatusLive:
		query = `UPDATE events SET status = $1, published_at = NOW(), paused_at = NULL WHERE id = $2`
	case models.EventStatusPaused:
		query = `UPDATE events SET status = $1, paused_at = NOW() WHERE id = $2`
	default:
		query = `UPDATE events SET status = $1 WHERE id = $2`
	}
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *postgresEventRepository) ListPublished(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	query := `SELECT ` + eventColumns + ` FROM events WHERE status = 'live' ORDER BY time ASC LIMIT $1 OFFSET $2`
	return r.scanEvents(ctx, query, limit, offset)
}

func (r *postgresEventRepository) scanEvents(ctx context.Context, query string, args ...interface{}) ([]*models.Event, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		e := &models.Event{}
		if err := rows.Scan(
			&e.ID, &e.HostID,
			&e.Title, &e.HookLine, &e.Mood, &e.Description,
			&e.CoverImageURL, &e.GalleryURLs,
			&e.IsOnline, &e.Location, &e.LocationLat, &e.LocationLng, &e.DurationMinutes, &e.MinGroupSize, &e.MaxGroupSize, &e.Capacity,
			&e.PriceCents, &e.IsFree, &e.Time, &e.EndTime, &e.IsRecurring, &e.RecurrenceRule,
			&e.CancellationPolicy, &e.Status, &e.PublishedAt, &e.PausedAt,
			&e.AISuggestion, &e.AvgRating, &e.TotalBookings,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		normalizedMood, err := models.NormalizeEventMood(e.Mood)
		if err != nil {
			return nil, err
		}
		e.Mood = normalizedMood
		events = append(events, e)
	}
	return events, nil
}
