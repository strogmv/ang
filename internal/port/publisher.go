package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type Publisher interface {
	PublishUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error
	PublishUserRegistered(ctx context.Context, event domain.UserRegistered) error
}
