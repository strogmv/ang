package service

import (
	"context"

	"github.com/example/minimal/internal/port"
)

type UserCached struct {
	base port.User
}

func NewUserCached(base port.User) *UserCached {
	return &UserCached{base: base}
}
func (c *UserCached) ListUsers(ctx context.Context, req port.ListUsersRequest) (port.ListUsersResponse, error) {
	return c.base.ListUsers(ctx, req)
}
