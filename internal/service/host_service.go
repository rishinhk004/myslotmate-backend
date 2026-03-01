package service

import (
	"context"
	"errors"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type HostService interface {
	CreateHost(ctx context.Context, userID uuid.UUID, name string, phnNumber string) (*models.Host, error)
	GetHostByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error)
}

type hostService struct {
	hostRepo   repository.HostRepository
	userRepo   repository.UserRepository
	dispatcher *event.Dispatcher
}

func NewHostService(hr repository.HostRepository, ur repository.UserRepository, d *event.Dispatcher) HostService {
	return &hostService{
		hostRepo:   hr,
		userRepo:   ur,
		dispatcher: d,
	}
}

func (s *hostService) CreateHost(ctx context.Context, userID uuid.UUID, name string, phnNumber string) (*models.Host, error) {
	// 1. Check if user exists and is verified
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if !user.IsVerified {
		return nil, errors.New("user is not verified")
	}

	// 2. Check if host already exists for this user
	existingHost, err := s.hostRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existingHost != nil {
		return nil, errors.New("host profile already exists for this user")
	}

	// 3. Create Host
	newHost := &models.Host{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		PhnNumber: phnNumber,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.hostRepo.Create(ctx, newHost); err != nil {
		return nil, err
	}

	// 4. Publish Event
	s.dispatcher.Publish(event.HostCreated, newHost)

	return newHost, nil
}

func (s *hostService) GetHostByUserID(ctx context.Context, userID uuid.UUID) (*models.Host, error) {
	return s.hostRepo.GetByUserID(ctx, userID)
}
