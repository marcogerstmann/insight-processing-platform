package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const (
	defaultMaxTokens = 512
	defaultTimeout   = 30 * time.Second
	enrichToolName   = "extract_enrichment"
)

var enrichTool = sdk.ToolUnionParam{
	OfTool: &sdk.ToolParam{
		Name:        enrichToolName,
		Description: sdk.Opt[string]("Extract thematic tags from a reading highlight."),
		InputSchema: sdk.ToolInputSchemaParam{
			Properties: map[string]any{
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "3-5 specific, thematic tags for the highlight. Prefer precise, reusable concepts (e.g. \"delegation\", \"compound-interest\") over generic ones (e.g. \"business\", \"life\").",
				},
			},
			Required: []string{"tags"},
		},
	},
}

var systemPrompt = []sdk.TextBlockParam{
	{Text: "You are a tagging specialist. Given a reading highlight, extract 3-5 precise thematic tags that capture its core concepts. Favor specific, reusable terms over broad categories. Be direct and concise. No preamble, no filler."},
}

type enrichmentInput struct {
	Tags []string `json:"tags"`
}

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

func (c *Client) Enrich(ctx context.Context, text string) (domain.Enrichment, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	msg, err := c.client.Messages.New(ctx, sdk.MessageNewParams{
		Model:      sdk.ModelClaudeHaiku4_5,
		MaxTokens:  c.maxTokens,
		System:     systemPrompt,
		Tools:      []sdk.ToolUnionParam{enrichTool},
		ToolChoice: sdk.ToolChoiceParamOfTool(enrichToolName),
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(text)),
		},
	})
	if err != nil {
		return domain.Enrichment{}, err
	}

	slog.InfoContext(ctx, "llm enrich complete",
		"model", msg.Model,
		"input_tokens", msg.Usage.InputTokens,
		"output_tokens", msg.Usage.OutputTokens,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	for _, block := range msg.Content {
		toolUse := block.AsToolUse()
		if toolUse.Name != enrichToolName {
			continue
		}

		var input enrichmentInput
		if err := json.Unmarshal(toolUse.Input, &input); err != nil {
			return domain.Enrichment{}, fmt.Errorf("unmarshal tool input: %w", err)
		}

		return domain.Enrichment{
			Tags: input.Tags,
		}, nil
	}

	return domain.Enrichment{}, errors.New("no tool use block in response")
}
