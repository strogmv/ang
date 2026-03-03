package port

import "context"

// TxManager defines the interface for managing database transactions.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
