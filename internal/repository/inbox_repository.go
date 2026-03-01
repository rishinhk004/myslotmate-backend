package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

type InboxRepository interface {
	Create(ctx context.Context, msg *models.InboxMessage) error
	ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.InboxMessage, error)
	ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.InboxMessage, error)
}

type postgresInboxRepository struct {
	db *sql.DB
}

func NewInboxRepository(db *sql.DB) InboxRepository {
	return &postgresInboxRepository{db: db}
}

func (r *postgresInboxRepository) Create(ctx context.Context, msg *models.InboxMessage) error {
	query := `
		INSERT INTO inbox_messages (id, event_id, host_id, message, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.EventID, msg.HostID, msg.Message, msg.CreatedAt,
	)
	return err
}

func (r *postgresInboxRepository) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.InboxMessage, error) {
	query := `SELECT id, event_id, host_id, message, created_at FROM inbox_messages WHERE event_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.InboxMessage
	for rows.Next() {
		m := &models.InboxMessage{}
		if err := rows.Scan(&m.ID, &m.EventID, &m.HostID, &m.Message, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *postgresInboxRepository) ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.InboxMessage, error) {
	query := `SELECT id, event_id, host_id, message, created_at FROM inbox_messages WHERE host_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.InboxMessage
	for rows.Next() {
		m := &models.InboxMessage{}
		if err := rows.Scan(&m.ID, &m.EventID, &m.HostID, &m.Message, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}
