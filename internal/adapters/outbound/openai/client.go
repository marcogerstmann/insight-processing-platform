// Package openai is the enrichment adapter — one bounded LLM call that
// turns a highlight into tags.
//
// Replaces the Anthropic adapter (IPP-135). The provider changed, not the
// architecture: this still satisfies ports.EnrichmentClient and nothing
// upstream of the port knows which vendor answers. The same key now also
// serves the AI service's embeddings, which is the whole point — see
// docs/adr/018-one-provider-for-model-capabilities.md.
//
// ADR-013's bounds are unchanged and deliberately visible here: a timeout
// per call, capped retries, and a hard output-token ceiling. Enrichment
// stays optional — the composition roots leave llm.Service nil when no key
// resolves, so the pipeline runs without it.
package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	sdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const (
	// enrichModel is the cheapest current-generation model that supports
	// structured outputs. Tagging one highlight is an easy extraction task
	// and does not need a frontier model; changing that judgement is this
	// one line.
	enrichModel = "gpt-5.6-luna"

	defaultMaxTokens = 512
	defaultTimeout   = 30 * time.Second
	schemaName       = "extract_enrichment"
)

// enrichSchema mirrors enrichmentInput.
//
// Structured outputs replace the forced tool call the Anthropic adapter
// used. Same guarantee — the model must return exactly this shape — but
// one response field instead of a tool definition plus a scan of the
// content blocks for the matching tool_use. `additionalProperties: false`
// and every property listed in `required` are what `strict` demands.
var enrichSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"tags": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "3-5 tags for the highlight, spanning a range of altitudes rather than 5 synonyms of the same idea. Include at least one broad field tag (e.g. \"psychology\", \"business\") the highlight clearly belongs to, then 2-4 tags that narrow into its specific facets (e.g. \"delayed-gratification\", \"locus-of-control\"). Never a narrow one-off tied to this highlight's exact wording (e.g. \"the-5-second-rule\", \"chapter-3-morning-routine\").",
		},
	},
	"required":             []string{"tags"},
	"additionalProperties": false,
}

const systemPrompt = "You are a tagging specialist. Given a reading highlight, extract 3-5 tags spanning a range of altitudes: start with one broad field it belongs to (e.g. \"psychology\", \"business\"), then add 2-4 tags that narrow into its specific facets. Never produce 5 tags that are all just synonyms of one idea, and never a narrow one-off tied to this highlight's exact wording. Be direct and concise. No preamble, no filler."

type enrichmentInput struct {
	Tags []string `json:"tags"`
}

type Client struct {
	client    sdk.Client
	model     string
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
		model:     enrichModel,
		maxTokens: defaultMaxTokens,
		timeout:   defaultTimeout,
	}
}

func (c *Client) Enrich(ctx context.Context, text string) (domain.Enrichment, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	completion, err := c.client.Chat.Completions.New(ctx, sdk.ChatCompletionNewParams{
		Model:               c.model,
		MaxCompletionTokens: sdk.Int(c.maxTokens),
		Messages: []sdk.ChatCompletionMessageParamUnion{
			sdk.SystemMessage(systemPrompt),
			sdk.UserMessage(text),
		},
		ResponseFormat: sdk.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   schemaName,
					Strict: sdk.Bool(true),
					Schema: enrichSchema,
				},
			},
		},
	})
	if err != nil {
		return domain.Enrichment{}, err
	}

	// Field names are load-bearing across both services — the Python
	// adapters match them so one Logs Insights query spans the pipeline
	// (IPP-113). Rename here only in lockstep with services/ai.
	slog.InfoContext(ctx, "llm enrich complete",
		"model", completion.Model,
		"input_tokens", completion.Usage.PromptTokens,
		"output_tokens", completion.Usage.CompletionTokens,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	if len(completion.Choices) == 0 {
		return domain.Enrichment{}, errors.New("no choices in response")
	}

	var input enrichmentInput
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &input); err != nil {
		return domain.Enrichment{}, fmt.Errorf("unmarshal structured output: %w", err)
	}

	return domain.Enrichment{
		Tags: input.Tags,
	}, nil
}
