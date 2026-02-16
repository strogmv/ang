package port

//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

// TagRepository defines storage operations for Tag
type TagRepository interface {
	Save(ctx context.Context, entity *domain.Tag) error
	FindByID(ctx context.Context, id string) (*domain.Tag, error)
	Delete(ctx context.Context, id string) error
	FindBySlug(ctx context.Context, slug string) (*domain.Tag, error)
	ListAll(ctx context.Context) ([]domain.Tag, error)
	ListByPost(ctx context.Context, id string) ([]domain.Tag, error)
}
