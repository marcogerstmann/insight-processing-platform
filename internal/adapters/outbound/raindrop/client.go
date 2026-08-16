// Package raindrop implements ports.HighlightSource against Raindrop.io's
// highlights API (https://developer.raindrop.io/v1/highlights), for
// bulk-importing and polling a tenant's highlights. Raindrop has no push
// webhook, so unlike readwise there is no sibling apigw package.
package raindrop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

const highlightsURL = "https://api.raindrop.io/rest/v1/highlights"

// EventType is stamped on every domain.IngestEvent produced from a Raindrop
// highlight, by both the REST import handler (rest/raindrop) and the
// scheduled poll (schedule/raindrop). Raindrop has no push webhook to match
// against, unlike Readwise's own webhook event_type, but the value must
// still be identical across both import paths so a highlight imported
// through either one hashes to the same idempotency key and dedupes against
// the other (see ingest.Importer's doc comment).
const EventType = "raindrop.highlight.created"

// perPage is Raindrop's documented maximum for this endpoint.
const perPage = 50

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    highlightsURL,
		token:      token,
	}
}

type highlightItem struct {
	ID      string    `json:"_id"`
	Text    string    `json:"text"`
	Note    string    `json:"note"`
	Link    string    `json:"link"`
	Created time.Time `json:"created"`
}

type highlightsResponse struct {
	Result bool            `json:"result"`
	Items  []highlightItem `json:"items"`
}

func (c *Client) FetchHighlights(ctx context.Context) ([]ports.SourceHighlight, error) {
	var out []ports.SourceHighlight

	// Raindrop's page param is 0-indexed; stop once a page comes back
	// shorter than perPage (including empty), since that's the last one.
	for page := 0; ; page++ {
		items, err := c.fetchPage(ctx, page)
		if err != nil {
			return nil, err
		}

		for _, h := range items {
			link := h.Link
			var urlPtr *string
			if link != "" {
				urlPtr = &link
			}
			out = append(out, ports.SourceHighlight{
				ID:            h.ID,
				Text:          h.Text,
				Note:          h.Note,
				URL:           urlPtr,
				HighlightedAt: h.Created,
				// Raindrop has no favourites concept.
				IsFavorite: false,
			})
		}

		if len(items) < perPage {
			break
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].HighlightedAt.After(out[j].HighlightedAt)
	})

	return out, nil
}

func (c *Client) fetchPage(ctx context.Context, page int) ([]highlightItem, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("perpage", strconv.Itoa(perPage))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, errors.New("raindrop: invalid API token")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("raindrop: rate limited, retry after %s", resp.Header.Get("Retry-After"))
	default:
		return nil, fmt.Errorf("raindrop: unexpected status %d", resp.StatusCode)
	}

	var body highlightsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Items, nil
}
