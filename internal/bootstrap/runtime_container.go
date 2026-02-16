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
	txManager := postgres.NewTxManager(pgPool)
	c.SvcAuth = service.NewAuthImpl(
		repoUser,
		publisher,
	)
	c.SvcBlog = service.NewBlogImpl(
		repoComment,
		repoPost,
		repoPostTag,
		repoTag,
		txManager,
	)

	return c, nil
}
