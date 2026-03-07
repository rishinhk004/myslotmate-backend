package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SupportRepository provides support ticket data access.
type SupportRepository interface {
	Create(ctx context.Context, ticket *models.SupportTicket) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SupportTicket, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.SupportTicket, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.SupportTicketStatus) error
	UpdateMessages(ctx context.Context, id uuid.UUID, messages []byte) error
}

type postgresSupportRepository struct {
	db *sql.DB
}

func NewSupportRepository(db *sql.DB) SupportRepository {
	return &postgresSupportRepository{db: db}
}

func (r *postgresSupportRepository) Create(ctx context.Context, ticket *models.SupportTicket) error {
	query := `
		INSERT INTO support_tickets (
			id, user_id, category, reported_user_id, subject, messages, status,
			event_id, session_date, report_reason, evidence_urls, is_urgent,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	`
	if ticket.ID == uuid.Nil {
		ticket.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		ticket.ID, ticket.UserID, ticket.Category, ticket.ReportedUserID,
		ticket.Subject, ticket.Messages, ticket.Status,
		ticket.EventID, ticket.SessionDate, ticket.ReportReason,
		pq.Array(ticket.EvidenceURLs), ticket.IsUrgent,
		ticket.CreatedAt, ticket.UpdatedAt,
	)
	return err
}

const supportTicketCols = `id, user_id, category, reported_user_id, subject, messages, status,
	event_id, session_date, report_reason, evidence_urls, is_urgent,
	created_at, updated_at`

func scanSupportTicket(row interface{ Scan(dest ...any) error }) (*models.SupportTicket, error) {
	t := &models.SupportTicket{}
	err := row.Scan(
		&t.ID, &t.UserID, &t.Category, &t.ReportedUserID,
		&t.Subject, &t.Messages, &t.Status,
		&t.EventID, &t.SessionDate, &t.ReportReason,
		pq.Array(&t.EvidenceURLs), &t.IsUrgent,
		&t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func (r *postgresSupportRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SupportTicket, error) {
	query := `SELECT ` + supportTicketCols + ` FROM support_tickets WHERE id = $1`
	t, err := scanSupportTicket(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (r *postgresSupportRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.SupportTicket, error) {
	query := `SELECT ` + supportTicketCols + ` FROM support_tickets WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []*models.SupportTicket
	for rows.Next() {
		t, err := scanSupportTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

func (r *postgresSupportRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.SupportTicketStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE support_tickets SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *postgresSupportRepository) UpdateMessages(ctx context.Context, id uuid.UUID, messages []byte) error {
	_, err := r.db.ExecContext(ctx, `UPDATE support_tickets SET messages = $1 WHERE id = $2`, messages, id)
	return err
}
