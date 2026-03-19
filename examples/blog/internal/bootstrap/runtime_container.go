package bootstrap

import (
	"context"

	"github.com/example/blog/internal/adapter/repository/postgres"
	"github.com/example/blog/internal/config"
	"github.com/example/blog/internal/port"
	"github.com/example/blog/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RuntimeContainer struct {
	Effects *EffectRegistry
	SvcAuth port.Auth
	SvcBlog port.Blog
}

func NewRuntimeContainer(
	ctx context.Context,
	cfg *config.Config,
	pgPool *pgxpool.Pool,
	publisher port.Publisher,
) (*RuntimeContainer, error) {
	_ = ctx
	c := &RuntimeContainer{}
	repoComment := postgres.NewCommentRepository(pgPool)
	repoPost := postgres.NewPostRepository(pgPool)
	repoPostTag := postgres.NewPostTagRepository(pgPool)
	repoTag := postgres.NewTagRepository(pgPool)
	repoUser := postgres.NewUserRepository(pgPool)
	effects, err := NewEffectRegistry(ctx, cfg, pgPool, publisher)
	if err != nil {
		return nil, err
	}
	c.Effects = effects
	txManager := postgres.NewTxManager(pgPool)
	c.SvcAuth = service.NewAuthImpl(
		repoUser,
		effects.Publisher,
	)
	c.SvcBlog = service.NewBlogImpl(
		repoComment,
		repoPost,
		repoPostTag,
		repoTag,
		txManager,
		effects.Publisher,
	)

	// ANG:BEGIN_CUSTOM runtime_container.after_init
	// Add project-specific runtime wiring here.
	// ANG:END_CUSTOM runtime_container.after_init

	return c, nil
}
