package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type AuthCached struct {
	base port.Auth
}

func NewAuthCached(base port.Auth) *AuthCached {
	return &AuthCached{base: base}
}
func (c *AuthCached) GetProfile(ctx context.Context, req port.GetProfileRequest) (port.GetProfileResponse, error) {
	return c.base.GetProfile(ctx, req)
}
func (c *AuthCached) Login(ctx context.Context, req port.LoginRequest) (port.LoginResponse, error) {
	return c.base.Login(ctx, req)
}
func (c *AuthCached) Register(ctx context.Context, req port.RegisterRequest) (port.RegisterResponse, error) {
	return c.base.Register(ctx, req)
}
func (c *AuthCached) UpdateProfile(ctx context.Context, req port.UpdateProfileRequest) (port.UpdateProfileResponse, error) {
	return c.base.UpdateProfile(ctx, req)
}
