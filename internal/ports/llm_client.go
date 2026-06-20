package ports

import "context"

type LLMClient interface {
	Prompt(ctx context.Context, prompt string) (string, error)
}
