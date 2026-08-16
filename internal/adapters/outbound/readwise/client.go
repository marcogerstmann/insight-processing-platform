// Package readwise implements ports.HighlightSource against Readwise's export
// API (https://readwise.io/api_deets), for bulk-importing a tenant's
// highlights. This is distinct from apigw/readwise, which handles Readwise's
// push webhook.
package readwise

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

const exportURL = "https://readwise.io/api/v2/export/"

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    exportURL,
		token:      token,
	}
}

type exportHighlight struct {
	ID            int64      `json:"id"`
	Text          string     `json:"text"`
	Note          string     `json:"note"`
	URL           *string    `json:"url"`
	HighlightedAt *time.Time `json:"highlighted_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	IsDeleted     bool       `json:"is_deleted"`
	IsFavorite    bool       `json:"is_favorite"`
}

type exportBook struct {
	Highlights []exportHighlight `json:"highlights"`
}

type exportResponse struct {
	NextPageCursor cursorValue  `json:"nextPageCursor"`
	Results        []exportBook `json:"results"`
}

// cursorValue holds Readwise's nextPageCursor value. Despite the API docs
// describing it as a string, Readwise has been observed returning it as a
// bare JSON number, so this accepts either representation.
type cursorValue string

func (c *cursorValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*c = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*c = cursorValue(s)
		return nil
	}
	*c = cursorValue(data)
	return nil
}

func (c *Client) FetchHighlights(ctx context.Context) ([]ports.SourceHighlight, error) {
	var out []ports.SourceHighlight
	cursor := ""

	for {
		page, err := c.fetchPage(ctx, cursor)
		if err != nil {
			return nil, err
		}

		for _, book := range page.Results {
			for _, h := range book.Highlights {
				if h.IsDeleted {
					continue
				}
				at := h.UpdatedAt
				if h.HighlightedAt != nil {
					at = *h.HighlightedAt
				}
				out = append(out, ports.SourceHighlight{
					ID:            strconv.FormatInt(h.ID, 10),
					Text:          h.Text,
					Note:          h.Note,
					URL:           h.URL,
					HighlightedAt: at,
					IsFavorite:    h.IsFavorite,
				})
			}
		}

		if page.NextPageCursor == "" {
			break
		}
		cursor = string(page.NextPageCursor)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].HighlightedAt.After(out[j].HighlightedAt)
	})

	return out, nil
}

func (c *Client) fetchPage(ctx context.Context, cursor string) (exportResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return exportResponse{}, err
	}
	if cursor != "" {
		q := u.Query()
		q.Set("pageCursor", cursor)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return exportResponse{}, err
	}
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return exportResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return exportResponse{}, errors.New("readwise: invalid API token")
	case http.StatusTooManyRequests:
		return exportResponse{}, fmt.Errorf("readwise: rate limited, retry after %s", resp.Header.Get("Retry-After"))
	default:
		return exportResponse{}, fmt.Errorf("readwise: unexpected status %d", resp.StatusCode)
	}

	var page exportResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return exportResponse{}, err
	}
	return page, nil
}
