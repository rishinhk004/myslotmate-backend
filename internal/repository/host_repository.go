package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

type HostRepository interface {
	Create(ctx context.Context, host *models.Host) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Host, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error)
}

type postgresHostRepository struct {
	db *sql.DB
}

func NewHostRepository(db *sql.DB) HostRepository {
	return &postgresHostRepository{db: db}
}

func (r *postgresHostRepository) Create(ctx context.Context, host *models.Host) error {
	query := `
		INSERT INTO hosts (id, user_id, name, phn_number, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if host.ID == uuid.Nil {
		host.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		host.ID, host.UserID, host.Name, host.PhnNumber, host.CreatedAt, host.UpdatedAt,
	)
	return err
}

func (r *postgresHostRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Host, error) {
	host := &models.Host{}
	query := `SELECT id, user_id, name, phn_number, created_at, updated_at FROM hosts WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&host.ID, &host.UserID, &host.Name, &host.PhnNumber, &host.CreatedAt, &host.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return host, nil
}

func (r *postgresHostRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error) {
	host := &models.Host{}
	query := `SELECT id, user_id, name, phn_number, created_at, updated_at FROM hosts WHERE user_id = $1`
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&host.ID, &host.UserID, &host.Name, &host.PhnNumber, &host.CreatedAt, &host.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return host, nil
}
