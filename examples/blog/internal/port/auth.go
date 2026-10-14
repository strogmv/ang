package port

import (
	"context"
)

type Auth interface {
	GetProfile(ctx context.Context, req GetProfileRequest) (GetProfileResponse, error)
	Login(ctx context.Context, req LoginRequest) (LoginResponse, error)
	OnUserLoggedInAudit(ctx context.Context, req OnUserLoggedInAuditRequest) (OnUserLoggedInAuditResponse, error)
	OnUserRegisteredAudit(ctx context.Context, req OnUserRegisteredAuditRequest) (OnUserRegisteredAuditResponse, error)
	Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error)
	UpdateProfile(ctx context.Context, req UpdateProfileRequest) (UpdateProfileResponse, error)
}

// Request/Response DTOs
type GetProfileRequest struct {
	UserID string `json:"userId"`
}

func (d *GetProfileRequest) Validate() error {
	return nil
}

type GetProfileResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	AvatarURL string `json:"avatarUrl"`
	CreatedAt string `json:"createdAt"`
}

func (d *GetProfileResponse) Validate() error {
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (d *LoginRequest) Validate() error {
	return nil
}

type LoginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         any    `json:"user"`
}

func (d *LoginResponse) Validate() error {
	return nil
}

type OnUserLoggedInAuditRequest struct {
	UserID string `json:"userId"`
}

func (d *OnUserLoggedInAuditRequest) Validate() error {
	return nil
}

type OnUserLoggedInAuditResponse struct {
	Ok bool `json:"ok"`
}

func (d *OnUserLoggedInAuditResponse) Validate() error {
	return nil
}

type OnUserRegisteredAuditRequest struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
}

func (d *OnUserRegisteredAuditRequest) Validate() error {
	return nil
}

type OnUserRegisteredAuditResponse struct {
	Ok bool `json:"ok"`
}

func (d *OnUserRegisteredAuditResponse) Validate() error {
	return nil
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (d *RegisterRequest) Validate() error {
	return nil
}

type RegisterResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (d *RegisterResponse) Validate() error {
	return nil
}

type UpdateProfileRequest struct {
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

func (d *UpdateProfileRequest) Validate() error {
	return nil
}

type UpdateProfileResponse struct {
	Ok bool `json:"ok"`
}

func (d *UpdateProfileResponse) Validate() error {
	return nil
}
