package anthropic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultMaxTokens = 150
	defaultTimeout   = 30 * time.Second
)

type Client struct {
	client    sdk.Client
	maxTokens int64
	timeout   time.Duration
}

func NewClient(apiKey string) *Client {
	client := sdk.NewClient(
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(3),
	)
	return &Client{
		client:    client,
		maxTokens: defaultMaxTokens,
		timeout:   defaultTimeout,
	}
}

func (c *Client) Prompt(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	msg, err := c.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:     sdk.ModelClaudeHaiku4_5,
		MaxTokens: c.maxTokens,
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", err
	}

	if len(msg.Content) == 0 {
		return "", errors.New("empty response from LLM")
	}

	slog.InfoContext(ctx, "llm prompt complete",
		"model", msg.Model,
		"input_tokens", msg.Usage.InputTokens,
		"output_tokens", msg.Usage.OutputTokens,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return msg.Content[0].AsText().Text, nil
}
