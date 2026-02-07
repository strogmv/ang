package port

//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks

import (
	"context"
	"github.com/strogmv/ang/internal/domain"
)

// PostRepository defines storage operations for Post
type PostRepository interface {
	Save(ctx context.Context, entity *domain.Post) error
	FindByID(ctx context.Context, id string) (*domain.Post, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context, offset, limit int) ([]domain.Post, error)

	// Dynamic Finders
	FindBySlug(ctx context.Context, slug string) (*domain.Post, error)
	ListPublished(ctx context.Context, status string) ([]domain.Post, error)
	CountPublished(ctx context.Context, status string) (int64, error)
	ListPublishedByTag(ctx context.Context, status string, id string) ([]domain.Post, error)
	CountPublishedByTag(ctx context.Context, status string, id string) (int64, error)
	ListByAuthor(ctx context.Context, authorID string) ([]domain.Post, error)
	ListByAuthorAndStatus(ctx context.Context, authorID string, status string) ([]domain.Post, error)
}
