package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/lib/identity"
	"myslotmate-backend/internal/lib/worker"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

// UserService defines the business logic interface
type UserService interface {
	SignUp(ctx context.Context, req SignUpRequest) (*models.User, error)
	GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req UserProfileUpdateRequest) (*models.User, error)
	InitiateAadharVerification(ctx context.Context, userID uuid.UUID, aadharNumber string) (string, error)
	CompleteAadharVerification(ctx context.Context, userID uuid.UUID, transactionID string, otp string) error
	SaveExperience(ctx context.Context, userID, eventID uuid.UUID) error
	UnsaveExperience(ctx context.Context, userID, eventID uuid.UUID) error
	GetSavedExperiences(ctx context.Context, userID uuid.UUID) ([]*models.SavedExperience, error)
	IsExperienceSaved(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

type SignUpRequest struct {
	AuthUID   string
	Email     string
	Name      string
	PhnNumber string
}

type UserProfileUpdateRequest struct {
	Name      *string `json:"name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	City      *string `json:"city,omitempty"`
}

// userService implements UserService
type userService struct {
	repo           repository.UserRepository
	savedExpRepo   repository.SavedExperienceRepository
	workerPool     *worker.WorkerPool
	dispatcher     *event.Dispatcher
	aadharProvider identity.AadharProvider
}

// NewUserService Factory for UserService
func NewUserService(repo repository.UserRepository, savedExpRepo repository.SavedExperienceRepository, wp *worker.WorkerPool, dispatcher *event.Dispatcher, ap identity.AadharProvider) UserService {
	return &userService{
		repo:           repo,
		savedExpRepo:   savedExpRepo,
		workerPool:     wp,
		dispatcher:     dispatcher,
		aadharProvider: ap,
	}
}

// InitiateAadharVerification starts the verification flow
func (s *userService) InitiateAadharVerification(ctx context.Context, userID uuid.UUID, aadharNumber string) (string, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}
	if user.IsVerified {
		return "", errors.New("user is already verified")
	}

	txnID, err := s.aadharProvider.InitiateVerification(ctx, aadharNumber)
	if err != nil {
		return "", err
	}

	return txnID, nil
}

// CompleteAadharVerification validates the OTP and marks user as verified
func (s *userService) CompleteAadharVerification(ctx context.Context, userID uuid.UUID, transactionID string, otp string) error {
	res, err := s.aadharProvider.VerifyOTP(ctx, transactionID, otp)
	if err != nil {
		return err
	}
	if !res.Success {
		return errors.New("verification failed by provider")
	}

	if err := s.repo.SetVerified(ctx, userID); err != nil {
		return err
	}

	// s.dispatcher.Publish("user_verified", userID)

	return nil
}

// SignUp handles user registration logic
func (s *userService) SignUp(ctx context.Context, req SignUpRequest) (*models.User, error) {
	if req.Email == "" {
		return nil, errors.New("email is required")
	}

	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("user already exists")
	}

	newUser := &models.User{
		ID:         uuid.New(),
		AuthUID:    req.AuthUID,
		Name:       req.Name,
		Email:      req.Email,
		PhnNumber:  req.PhnNumber,
		IsVerified: false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, newUser); err != nil {
		return nil, err
	}

	// Publish Event (Observer Pattern) - "User Created"
	// This helps decouple this service from email service, analytics, etc.
	s.dispatcher.Publish(event.UserCreated, newUser)

	// Execute Background Task (Executor/Worker Pattern) - Example: Send Welcome Email
	// If you want explicit background task here (alternative to observing the event):
	s.workerPool.Submit(func() {
		// Simulate sending email
		fmt.Printf("Sending welcome email to %s (User ID: %s)\n", newUser.Email, newUser.ID)
		time.Sleep(2 * time.Second) // Simulate network delay
		fmt.Printf("Email sent to %s\n", newUser.Email)
	})

	return newUser, nil
}

func (s *userService) GetProfile(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID uuid.UUID, req UserProfileUpdateRequest) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	if req.City != nil {
		user.City = req.City
	}
	user.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) SaveExperience(ctx context.Context, userID, eventID uuid.UUID) error {
	se := &models.SavedExperience{
		ID:      uuid.New(),
		UserID:  userID,
		EventID: eventID,
		SavedAt: time.Now(),
	}
	return s.savedExpRepo.Save(ctx, se)
}

func (s *userService) UnsaveExperience(ctx context.Context, userID, eventID uuid.UUID) error {
	return s.savedExpRepo.Remove(ctx, userID, eventID)
}

func (s *userService) GetSavedExperiences(ctx context.Context, userID uuid.UUID) ([]*models.SavedExperience, error) {
	return s.savedExpRepo.ListByUserID(ctx, userID)
}

func (s *userService) IsExperienceSaved(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	return s.savedExpRepo.Exists(ctx, userID, eventID)
}
