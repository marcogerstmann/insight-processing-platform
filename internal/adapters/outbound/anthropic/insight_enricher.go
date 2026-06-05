package anthropic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const (
	defaultMaxTokens = 150
	defaultTimeout   = 30 * time.Second

	prompt = `You are an insight extractor. Given a reading highlight, write 1-2 sentences that capture the core idea or key takeaway. Be direct and concise. No preamble, no filler.

Highlight: %s`
)

type InsightEnricher struct {
	client    sdk.Client
	maxTokens int64
	timeout   time.Duration
}

func NewInsightEnricher(apiKey string) *InsightEnricher {
	client := sdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(3),
	)
	return &InsightEnricher{
		client:    client,
		maxTokens: defaultMaxTokens,
		timeout:   defaultTimeout,
	}
}

func (e *InsightEnricher) Enrich(ctx context.Context, insight domain.Insight) (domain.Insight, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	start := time.Now()
	msg, err := e.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.ModelClaudeHaiku4_5,
		MaxTokens: e.maxTokens,
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(fmt.Sprintf(prompt, insight.Text))),
		},
	})
	if err != nil {
		return domain.Insight{}, err
	}

	if len(msg.Content) == 0 {
		return domain.Insight{}, errors.New("empty response from LLM")
	}

	slog.InfoContext(ctx, "llm enrichment complete",
		"insight_id", insight.ID,
		"model", msg.Model,
		"input_tokens", msg.Usage.InputTokens,
		"output_tokens", msg.Usage.OutputTokens,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	insight.Summary = msg.Content[0].AsText().Text
	return insight, nil
}
