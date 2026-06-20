package llm

import (
	"context"
	"fmt"

	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

const summarizePrompt = `You are an insight extractor. Given a reading highlight, write 1-2 sentences that capture the core idea or key takeaway. Be direct and concise. No preamble, no filler.

Highlight: %s`

type Service struct {
	client ports.LLMClient
}

func NewService(client ports.LLMClient) *Service {
	return &Service{client: client}
}

func (s *Service) Summarize(ctx context.Context, text string) (string, error) {
	return s.client.Prompt(ctx, fmt.Sprintf(summarizePrompt, text))
}
