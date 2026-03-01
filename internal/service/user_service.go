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
	InitiateAadharVerification(ctx context.Context, userID uuid.UUID, aadharNumber string) (string, error)
	CompleteAadharVerification(ctx context.Context, userID uuid.UUID, transactionID string, otp string) error
}

type SignUpRequest struct {
	AuthUID   string
	Email     string
	Name      string
	PhnNumber string
}

// userService implements UserService
type userService struct {
	repo           repository.UserRepository
	workerPool     *worker.WorkerPool
	dispatcher     *event.Dispatcher
	aadharProvider identity.AadharProvider
}

// NewUserService Factory for UserService
// Dependency Injection: Inject Repository, WorkerPool, Dispatcher (Singleton usually), AadharProvider
func NewUserService(repo repository.UserRepository, wp *worker.WorkerPool, dispatcher *event.Dispatcher, ap identity.AadharProvider) UserService {
	return &userService{
		repo:           repo,
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
