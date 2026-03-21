package bootstrap

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/strogmv/ang/internal/adapter/repository/postgres"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/port"
	"github.com/strogmv/ang/internal/service"
)

type RuntimeContainer struct {
	Effects          *EffectRegistry
	SvcAuth          port.Auth
	SvcBlog          port.Blog
	SvcAssistant     port.Assistant
	SvcNotifications port.Notifications
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
	repoComment := postgres.NewCommentRepository(pgPool)
	repoPost := postgres.NewPostRepository(pgPool)
	repoPostTag := postgres.NewPostTagRepository(pgPool)
	repoTag := postgres.NewTagRepository(pgPool)
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
	txManager := postgres.NewTxManager(pgPool)
	c.SvcAuth = service.NewAuthImpl(
		repoUser,
		effects.Publisher,
		notificationDispatcher,
	)
	c.SvcBlog = service.NewBlogImpl(
		repoComment,
		repoPost,
		repoPostTag,
		repoTag,
		c.SvcAuth,
		txManager,
		effects.Publisher,
	)
	c.SvcAssistant = service.NewAssistantImpl(
		repoPost,
		c.SvcAuth,
		c.SvcBlog,
		effects.StateStore,
	)
	c.SvcNotifications = service.NewNotificationsImpl(
		c.SvcAuth,
		notificationDispatcher,
	)

	// ANG:BEGIN_CUSTOM runtime_container.after_init
	// Add project-specific runtime wiring here.
	// ANG:END_CUSTOM runtime_container.after_init

	return c, nil
}
