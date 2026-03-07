package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type HostRepository interface {
	Create(ctx context.Context, host *models.Host) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Host, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error)
	Update(ctx context.Context, host *models.Host) error
	UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status models.HostApplicationStatus) error
	ListByStatus(ctx context.Context, status models.HostApplicationStatus) ([]*models.Host, error)
}

type postgresHostRepository struct {
	db *sql.DB
}

func NewHostRepository(db *sql.DB) HostRepository {
	return &postgresHostRepository{db: db}
}

var hostColumns = `id, user_id, account_id,
	first_name, last_name, phn_number, city, avatar_url, tagline, bio,
	application_status, experience_desc, moods, description, preferred_days, group_size,
	government_id_url, submitted_at, approved_at, rejected_at,
	is_identity_verified, is_email_verified, is_phone_verified, is_super_host, is_community_champ,
	expertise_tags, social_instagram, social_linkedin, social_website,
	avg_rating, total_reviews,
	created_at, updated_at`

func scanHost(row interface {
	Scan(dest ...interface{}) error
}) (*models.Host, error) {
	h := &models.Host{}
	err := row.Scan(
		&h.ID, &h.UserID, &h.AccountID,
		&h.FirstName, &h.LastName, &h.PhnNumber, &h.City, &h.AvatarURL, &h.Tagline, &h.Bio,
		&h.ApplicationStatus, &h.ExperienceDesc, &h.Moods, &h.Description, &h.PreferredDays, &h.GroupSize,
		&h.GovernmentIDURL, &h.SubmittedAt, &h.ApprovedAt, &h.RejectedAt,
		&h.IsIdentityVerified, &h.IsEmailVerified, &h.IsPhoneVerified, &h.IsSuperHost, &h.IsCommunityChamp,
		&h.ExpertiseTags, &h.SocialInstagram, &h.SocialLinkedin, &h.SocialWebsite,
		&h.AvgRating, &h.TotalReviews,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return h, nil
}

func (r *postgresHostRepository) Create(ctx context.Context, host *models.Host) error {
	query := `
		INSERT INTO hosts (
			id, user_id,
			first_name, last_name, phn_number, city, avatar_url, tagline, bio,
			application_status, experience_desc, moods, description, preferred_days, group_size,
			government_id_url, submitted_at,
			expertise_tags, social_instagram, social_linkedin, social_website,
			created_at, updated_at
		) VALUES (
			$1, $2,
			$3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15,
			$16, $17,
			$18, $19, $20, $21,
			$22, $23
		)
	`
	if host.ID == uuid.Nil {
		host.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		host.ID, host.UserID,
		host.FirstName, host.LastName, host.PhnNumber, host.City, host.AvatarURL, host.Tagline, host.Bio,
		host.ApplicationStatus, host.ExperienceDesc, pq.Array(host.Moods), host.Description, pq.Array(host.PreferredDays), host.GroupSize,
		host.GovernmentIDURL, host.SubmittedAt,
		pq.Array(host.ExpertiseTags), host.SocialInstagram, host.SocialLinkedin, host.SocialWebsite,
		host.CreatedAt, host.UpdatedAt,
	)
	return err
}

func (r *postgresHostRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Host, error) {
	query := `SELECT ` + hostColumns + ` FROM hosts WHERE id = $1`
	return scanHost(r.db.QueryRowContext(ctx, query, id))
}

func (r *postgresHostRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error) {
	query := `SELECT ` + hostColumns + ` FROM hosts WHERE user_id = $1`
	return scanHost(r.db.QueryRowContext(ctx, query, userID))
}

func (r *postgresHostRepository) Update(ctx context.Context, host *models.Host) error {
	query := `
		UPDATE hosts SET
			first_name = $1, last_name = $2, phn_number = $3, city = $4, avatar_url = $5, tagline = $6, bio = $7,
			application_status = $8, experience_desc = $9, moods = $10, description = $11, preferred_days = $12, group_size = $13,
			government_id_url = $14, submitted_at = $15, approved_at = $16, rejected_at = $17,
			is_identity_verified = $18, is_email_verified = $19, is_phone_verified = $20, is_super_host = $21, is_community_champ = $22,
			expertise_tags = $23, social_instagram = $24, social_linkedin = $25, social_website = $26,
			avg_rating = $27, total_reviews = $28
		WHERE id = $29
	`
	_, err := r.db.ExecContext(ctx, query,
		host.FirstName, host.LastName, host.PhnNumber, host.City, host.AvatarURL, host.Tagline, host.Bio,
		host.ApplicationStatus, host.ExperienceDesc, pq.Array(host.Moods), host.Description, pq.Array(host.PreferredDays), host.GroupSize,
		host.GovernmentIDURL, host.SubmittedAt, host.ApprovedAt, host.RejectedAt,
		host.IsIdentityVerified, host.IsEmailVerified, host.IsPhoneVerified, host.IsSuperHost, host.IsCommunityChamp,
		pq.Array(host.ExpertiseTags), host.SocialInstagram, host.SocialLinkedin, host.SocialWebsite,
		host.AvgRating, host.TotalReviews,
		host.ID,
	)
	return err
}

func (r *postgresHostRepository) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status models.HostApplicationStatus) error {
	query := `UPDATE hosts SET application_status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *postgresHostRepository) ListByStatus(ctx context.Context, status models.HostApplicationStatus) ([]*models.Host, error) {
	query := `SELECT ` + hostColumns + ` FROM hosts WHERE application_status = $1 ORDER BY submitted_at DESC`
	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []*models.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}
