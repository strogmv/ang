package service

import (
	"context"

	"github.com/strogmv/ang/internal/pkg/helpers"
	"github.com/strogmv/ang/internal/port"
)

type AuthImpl struct {
	UserRepo  port.UserRepository
	publisher port.Publisher
}

func NewAuthImpl(userRepo port.UserRepository, publisher port.Publisher) *AuthImpl {
	return &AuthImpl{UserRepo: userRepo, publisher: publisher}
}

func (s *AuthImpl) GetProfile(ctx context.Context, req port.GetProfileRequest) (resp port.GetProfileResponse, err error) {
	user, err := s.UserRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.ID, user.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Email, user.Email); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Name, user.Name); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Role, user.Role); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.AvatarURL, user.AvatarURL); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.CreatedAt, user.CreatedAt); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *AuthImpl) Login(ctx context.Context, req port.LoginRequest) (resp port.LoginResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *AuthImpl) Register(ctx context.Context, req port.RegisterRequest) (resp port.RegisterResponse, err error) {
	// FLOW_NOT_IMPLEMENTED: unsupported flow actions in this method, fallback stub.
	return resp, nil
}

func (s *AuthImpl) UpdateProfile(ctx context.Context, req port.UpdateProfileRequest) (resp port.UpdateProfileResponse, err error) {
	user, err := s.UserRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return resp, err
	}
	if req.Name != "" {
		if err := helpers.Assign(&user.Name, req.Name); err != nil {
			return resp, err
		}
	}
	if req.AvatarURL != "" {
		if err := helpers.Assign(&user.AvatarURL, req.AvatarURL); err != nil {
			return resp, err
		}
	}
	if err := s.UserRepo.Save(ctx, user); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Ok, true); err != nil {
		return resp, err
	}

	return resp, nil
}
