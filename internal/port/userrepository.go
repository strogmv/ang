package port
//go:generate go run go.uber.org/mock/mockgen@latest -source=$GOFILE -destination=mocks/mock_$GOFILE -package=mocks
import (
	"context"
	"github.com/strogmv/ang/internal/domain"
)
// UserRepository defines storage operations for User
type UserRepository interface {
	Save(ctx context.Context, entity *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	FindByEmail(ctx context.Context, email map[string]any) (*domain.User, error)
	ListAll(ctx context.Context, offset int, limit int) ([]domain.User, error)
}
