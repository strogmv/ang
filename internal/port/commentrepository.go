package port
//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
import (
	"context"
	"github.com/strogmv/ang/internal/domain"
)
// CommentRepository defines storage operations for Comment
type CommentRepository interface {
	Save(ctx context.Context, entity *domain.Comment) error
	FindByID(ctx context.Context, id string) (*domain.Comment, error)
	Delete(ctx context.Context, id string) error
	CountByPost(ctx context.Context, postID string) (int64, error)
	DeleteByParent(ctx context.Context, parentID string) (int64, error)
	DeleteByPost(ctx context.Context, postID string) (int64, error)
	ListByPost(ctx context.Context, postID string) ([]domain.Comment, error)
	ListAll(ctx context.Context, offset int, limit int) ([]domain.Comment, error)
}
