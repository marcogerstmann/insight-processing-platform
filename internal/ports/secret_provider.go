package ports

import "context"

type SecretProvider interface {
	Get(ctx context.Context, name string) (string, error)
}
