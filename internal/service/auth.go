package service

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/strogmv/ang/internal/domain"
	"github.com/strogmv/ang/internal/pkg/errors"
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
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "User not found")
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
	user, err := s.UserRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return resp, err
	}
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "Invalid credentials")
	}
	valid, err := checkPassword(req.Password, user.PasswordHash)
	if err != nil {
		return resp, err
	}
	if !(valid) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Invalid credentials")
	}
	tokens, err := generateTokens(user)
	if err != nil {
		return resp, err
	}
	if s.publisher != nil {
		_ = s.publisher.PublishUserLoggedIn(ctx, domain.UserLoggedIn{UserID: user.ID})
	}
	if err := helpers.Assign(&resp.AccessToken, tokens.AccessToken); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.RefreshToken, tokens.RefreshToken); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.User.ID, user.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.User.Email, user.Email); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.User.Name, user.Name); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.User.Role, user.Role); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *AuthImpl) Register(ctx context.Context, req port.RegisterRequest) (resp port.RegisterResponse, err error) {
	existing, err := s.UserRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return resp, err
	}
	if !(existing == nil) {
		return resp, errors.New(http.StatusBadRequest, "Validation Error", "Email already registered")
	}
	var newUser domain.User
	if err := helpers.Assign(&newUser.Email, req.Email); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&newUser.Name, req.Name); err != nil {
		return resp, err
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		return resp, err
	}
	if err := helpers.Assign(&newUser.PasswordHash, hash); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&newUser.Role, "reader"); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&newUser.ID, uuid.NewString()); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&newUser.CreatedAt, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return resp, err
	}
	if err := s.UserRepo.Save(ctx, &newUser); err != nil {
		return resp, err
	}
	if s.publisher != nil {
		_ = s.publisher.PublishUserRegistered(ctx, domain.UserRegistered{UserID: newUser.ID, Email: newUser.Email})
	}
	if err := helpers.Assign(&resp.ID, newUser.ID); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Email, newUser.Email); err != nil {
		return resp, err
	}
	if err := helpers.Assign(&resp.Name, newUser.Name); err != nil {
		return resp, err
	}

	return resp, nil
}

func (s *AuthImpl) UpdateProfile(ctx context.Context, req port.UpdateProfileRequest) (resp port.UpdateProfileResponse, err error) {
	user, err := s.UserRepo.FindByID(ctx, req.UserID)
	if err != nil {
		return resp, err
	}
	if user == nil {
		return resp, errors.New(http.StatusNotFound, "Not Found", "User not found")
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
