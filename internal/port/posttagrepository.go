package port

//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks

import (
	"context"
	"github.com/strogmv/ang/internal/domain"
)

// PostTagRepository defines storage operations for PostTag
type PostTagRepository interface {
	Save(ctx context.Context, entity *domain.PostTag) error
	FindByID(ctx context.Context, id string) (*domain.PostTag, error)
	Delete(ctx context.Context, id string) error
	ListAll(ctx context.Context, offset, limit int) ([]domain.PostTag, error)

	// Dynamic Finders
	DeleteByPost(ctx context.Context, postID string) (int64, error)
	DeleteByTag(ctx context.Context, tagID string) (int64, error)
}
