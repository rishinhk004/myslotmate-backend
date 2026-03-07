package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Host is a verified user who can create events, see analytics, and manage payouts.
type Host struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	AccountID *uuid.UUID `db:"account_id" json:"account_id,omitempty"`

	// ── Personal / Profile ──────────────────────────────────────────────────
	FirstName string  `db:"first_name" json:"first_name"`
	LastName  string  `db:"last_name" json:"last_name"`
	PhnNumber string  `db:"phn_number" json:"phn_number"`
	City      string  `db:"city" json:"city"`
	AvatarURL *string `db:"avatar_url" json:"avatar_url,omitempty"`
	Tagline   *string `db:"tagline" json:"tagline,omitempty"`
	Bio       *string `db:"bio" json:"bio,omitempty"`

	// ── Host Application (Become a Host flow) ───────────────────────────────
	ApplicationStatus HostApplicationStatus `db:"application_status" json:"application_status"`
	ExperienceDesc    *string               `db:"experience_desc" json:"experience_desc,omitempty"`     // "What Experiences will you Host?"
	Moods             pq.StringArray        `db:"moods" json:"moods"`                                   // ["adventure","social","wellness"]
	Description       *string               `db:"description" json:"description,omitempty"`             // 300 char description
	PreferredDays     pq.StringArray        `db:"preferred_days" json:"preferred_days"`                 // ["mon","tue","wed"]
	GroupSize         *int                  `db:"group_size" json:"group_size,omitempty"`               // approximate group size
	GovernmentIDURL   *string               `db:"government_id_url" json:"government_id_url,omitempty"` // uploaded ID doc URL
	SubmittedAt       *time.Time            `db:"submitted_at" json:"submitted_at,omitempty"`
	ApprovedAt        *time.Time            `db:"approved_at" json:"approved_at,omitempty"`
	RejectedAt        *time.Time            `db:"rejected_at" json:"rejected_at,omitempty"`

	// ── Trust & Safety badges ───────────────────────────────────────────────
	IsIdentityVerified bool `db:"is_identity_verified" json:"is_identity_verified"`
	IsEmailVerified    bool `db:"is_email_verified" json:"is_email_verified"`
	IsPhoneVerified    bool `db:"is_phone_verified" json:"is_phone_verified"`
	IsSuperHost        bool `db:"is_super_host" json:"is_super_host"`
	IsCommunityChamp   bool `db:"is_community_champ" json:"is_community_champ"`

	// ── Expertise & Social ──────────────────────────────────────────────────
	ExpertiseTags   pq.StringArray `db:"expertise_tags" json:"expertise_tags"` // ["#Minimalism","#Wellness"]
	SocialInstagram *string        `db:"social_instagram" json:"social_instagram,omitempty"`
	SocialLinkedin  *string        `db:"social_linkedin" json:"social_linkedin,omitempty"`
	SocialWebsite   *string        `db:"social_website" json:"social_website,omitempty"`

	// ── Aggregate stats (denormalized for dashboard) ────────────────────────
	AvgRating    *float64 `db:"avg_rating" json:"avg_rating,omitempty"`
	TotalReviews int      `db:"total_reviews" json:"total_reviews"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
