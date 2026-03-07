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
	MarkRead(ctx context.Context, id uuid.UUID) error
}

type postgresInboxRepository struct {
	db *sql.DB
}

func NewInboxRepository(db *sql.DB) InboxRepository {
	return &postgresInboxRepository{db: db}
}

var inboxColumns = `id, event_id, sender_type, sender_id, message, attachment_url, is_read, created_at`

func scanInboxMessage(row interface {
	Scan(dest ...interface{}) error
}) (*models.InboxMessage, error) {
	m := &models.InboxMessage{}
	err := row.Scan(&m.ID, &m.EventID, &m.SenderType, &m.SenderID, &m.Message, &m.AttachmentURL, &m.IsRead, &m.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

func (r *postgresInboxRepository) Create(ctx context.Context, msg *models.InboxMessage) error {
	query := `
		INSERT INTO inbox_messages (id, event_id, sender_type, sender_id, message, attachment_url, is_read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		msg.ID, msg.EventID, msg.SenderType, msg.SenderID, msg.Message, msg.AttachmentURL, msg.IsRead, msg.CreatedAt,
	)
	return err
}

func (r *postgresInboxRepository) ListByEventID(ctx context.Context, eventID uuid.UUID) ([]*models.InboxMessage, error) {
	query := `SELECT ` + inboxColumns + ` FROM inbox_messages WHERE event_id = $1 ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.InboxMessage
	for rows.Next() {
		m := &models.InboxMessage{}
		if err := rows.Scan(&m.ID, &m.EventID, &m.SenderType, &m.SenderID, &m.Message, &m.AttachmentURL, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *postgresInboxRepository) ListByHostID(ctx context.Context, hostID uuid.UUID) ([]*models.InboxMessage, error) {
	// Get all messages for events owned by this host
	query := `SELECT im.` + inboxColumns + `
		FROM inbox_messages im
		JOIN events e ON e.id = im.event_id
		WHERE e.host_id = $1
		ORDER BY im.created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*models.InboxMessage
	for rows.Next() {
		m := &models.InboxMessage{}
		if err := rows.Scan(&m.ID, &m.EventID, &m.SenderType, &m.SenderID, &m.Message, &m.AttachmentURL, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *postgresInboxRepository) MarkRead(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbox_messages SET is_read = true WHERE id = $1`, id)
	return err
}
