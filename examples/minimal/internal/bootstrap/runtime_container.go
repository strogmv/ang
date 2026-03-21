package bootstrap

import (
	"context"

	"github.com/example/minimal/internal/adapter/repository/postgres"
	"github.com/example/minimal/internal/config"
	"github.com/example/minimal/internal/port"
	"github.com/example/minimal/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RuntimeContainer struct {
	Effects          *EffectRegistry
	SvcNotifications port.Notifications
	SvcUser          port.User
}

func NewRuntimeContainer(
	ctx context.Context,
	cfg *config.Config,
	pgPool *pgxpool.Pool,
	publisher port.Publisher,
	notificationDispatcher port.NotificationDispatcher,
) (*RuntimeContainer, error) {
	_ = ctx
	c := &RuntimeContainer{}
	repoUser := postgres.NewUserRepository(pgPool)
	effects, err := NewEffectRegistry(
		ctx,
		cfg,
		pgPool,
		publisher,
		notificationDispatcher,
	)
	if err != nil {
		return nil, err
	}
	c.Effects = effects
	c.SvcNotifications = service.NewNotificationsImpl(
		notificationDispatcher,
	)
	c.SvcUser = service.NewUserImpl(
		repoUser,
	)

	// ANG:BEGIN_CUSTOM runtime_container.after_init
	// Add project-specific runtime wiring here.
	// ANG:END_CUSTOM runtime_container.after_init

	return c, nil
}
