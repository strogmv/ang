package bootstrap

import (
	"context"

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
	publisher port.Publisher,
) (*RuntimeContainer, error) {
	_ = ctx
	c := &RuntimeContainer{}
	repoComment := postgres.NewCommentRepository(pgPool)
	repoPost := postgres.NewPostRepository(pgPool)
	repoPostTag := postgres.NewPostTagRepository(pgPool)
	repoTag := postgres.NewTagRepository(pgPool)
	repoUser := postgres.NewUserRepository(pgPool)
	repoUserVault := postgres.NewUserVaultRepository(pgPool)
	txManager := postgres.NewTxManager(pgPool)

	svcAudit := service.NewAuditImpl(repoAuditLog)
	c.SvcAudit = svcAudit
	c.SvcAuth = service.NewAuthImpl(
		repoUser,
		publisher,
		svcAudit,
	)
	c.SvcBlog = service.NewBlogImpl(
		repoComment,
		repoPost,
		repoPostTag,
		repoTag,
		txManager,
		svcAudit,
	)

	return c, nil
}
