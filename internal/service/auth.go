package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type AuthImpl struct {
	UserRepo     port.UserRepository
	publisher    port.Publisher
	auditService port.Audit
}

func NewAuthImpl(userRepo port.UserRepository, publisher port.Publisher, auditService port.Audit) *AuthImpl {
	return &AuthImpl{UserRepo: userRepo, publisher: publisher, auditService: auditService}
}

func (s *AuthImpl) GetProfile(ctx context.Context, req port.GetProfileRequest) (resp port.GetProfileResponse, err error) {
	return resp, nil
}

func (s *AuthImpl) Login(ctx context.Context, req port.LoginRequest) (resp port.LoginResponse, err error) {
	return resp, nil
}

func (s *AuthImpl) Register(ctx context.Context, req port.RegisterRequest) (resp port.RegisterResponse, err error) {
	return resp, nil
}

func (s *AuthImpl) UpdateProfile(ctx context.Context, req port.UpdateProfileRequest) (resp port.UpdateProfileResponse, err error) {
	return resp, nil
}
