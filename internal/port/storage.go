package port

import (
	"context"
	"io"
	"time"
)

type FileStorage interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	GetURL(ctx context.Context, key string) (string, error)
	PresignGet(ctx context.Context, key string, expiresIn time.Duration) (string, error)
}
