package app

import (
	"context"

	"github.com/strogmv/ang/internal/adapter/repository/postgres"
	"github.com/strogmv/ang/internal/port"
	"github.com/strogmv/ang/internal/service"
	"github.com/strogmv/ang/internal/config"
	"github.com/jmoiron/sqlx"
)

type Container struct {
	Config *config.Config
	DB     *sqlx.DB

	
	RepoUser port.UserRepository
	RepoPost port.PostRepository
	RepoComment port.CommentRepository
	RepoTag port.TagRepository
	RepoPostTag port.PostTagRepository
	RepoUserVault port.UserVaultRepository

	
	SvcAuth port.Auth
	SvcBlog port.Blog
}

func NewContainer(ctx context.Context, cfg *config.Config, db *sqlx.DB, extra ...any) (*Container, error) {
	c := &Container{
		Config: cfg,
		DB:     db,
	}

	
	c.RepoUser = postgres.NewUserRepository(db)
	c.RepoPost = postgres.NewPostRepository(db)
	c.RepoComment = postgres.NewCommentRepository(db)
	c.RepoTag = postgres.NewTagRepository(db)
	c.RepoPostTag = postgres.NewPostTagRepository(db)
	c.RepoUserVault = postgres.NewUserVaultRepository(db)

	var svcAudit port.Audit
	

	
	
	c.SvcAuth = service.NewAuthImpl(
		c.RepoUser, c.RepoPost, c.RepoComment, c.RepoTag, c.RepoPostTag, c.RepoUserVault, 
		svcAudit,
		extra...,
	)
	
	
	c.SvcBlog = service.NewBlogImpl(
		c.RepoUser, c.RepoPost, c.RepoComment, c.RepoTag, c.RepoPostTag, c.RepoUserVault, 
		svcAudit,
		extra...,
	)
	

	return c, nil
}
