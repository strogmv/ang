package port

import (
	"context"

	"github.com/example/minimal/internal/domain"
)

type UserRepositoryMock struct {
	FindByIDFunc func(ctx context.Context, id string) (*domain.User, error)
	SaveFunc     func(ctx context.Context, item *domain.User) error
	DeleteFunc   func(ctx context.Context, id string) error
	InsertFunc   func(ctx context.Context, item *domain.User) error
	UpdateFunc   func(ctx context.Context, item *domain.User) error
	LockByIDFunc func(ctx context.Context, id string) (*domain.User, error)
	ListFunc     func(ctx context.Context) (any, error)
}

func (m *UserRepositoryMock) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *UserRepositoryMock) Save(ctx context.Context, item *domain.User) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *UserRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *UserRepositoryMock) Insert(ctx context.Context, item *domain.User) error {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, item)
	}
	return nil
}

func (m *UserRepositoryMock) Update(ctx context.Context, item *domain.User) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, item)
	}
	return nil
}

func (m *UserRepositoryMock) LockByID(ctx context.Context, id string) (*domain.User, error) {
	if m.LockByIDFunc != nil {
		return m.LockByIDFunc(ctx, id)
	}
	return nil, nil
}
func (m *UserRepositoryMock) List(ctx context.Context) (any, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func NewUserRepositoryMock() *UserRepositoryMock {
	return &UserRepositoryMock{}
}
