package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const metasoBaseURL = "https://metaso.cn/api/v1"

// MetasoClient is an HTTP client for Metaso search and reader APIs.
type MetasoClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// NewMetasoClient creates a Metaso search/reader client.
func NewMetasoClient(apiKey string) *MetasoClient {
	return &MetasoClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    metasoBaseURL,
	}
}

// Search executes a webpage search through Metaso.
func (c *MetasoClient) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if c == nil || c.apiKey == "" {
		return nil, fmt.Errorf("metaso not configured")
	}
	if strings.TrimSpace(req.Query) == "" {
		return nil, fmt.Errorf("search: query is required")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}

	payload := map[string]any{
		"q":                 req.Query,
		"scope":             "webpage",
		"includeSummary":    req.IncludeAnswer,
		"size":              strconv.Itoa(maxResults),
		"includeRawContent": req.IncludeRaw,
		"conciseSnippet":    false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal metaso search request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create metaso search request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("metaso search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read metaso search response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &SearchError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Metaso API error %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	var raw any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal metaso search response: %w", err)
	}
	result := normalizeMetasoSearchResult(req.Query, raw, maxResults)
	return result, nil
}

// Fetch reads a webpage through the Metaso reader API.
func (c *MetasoClient) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	if c == nil || c.apiKey == "" {
		return nil, fmt.Errorf("metaso not configured")
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("fetch: url is required")
	}

	payload := map[string]any{"url": req.URL}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal metaso reader request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL, "/")+"/reader", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create metaso reader request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/plain")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("metaso reader request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read metaso reader response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &SearchError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Metaso Reader API error %d: %s", resp.StatusCode, string(respBody)),
		}
	}

	content, title := normalizeMetasoReaderContent(respBody)
	return &FetchResult{
		Success: true,
		Data: FetchData{
			Markdown: content,
			Metadata: FetchMetadata{
				Title:      title,
				SourceURL:  req.URL,
				StatusCode: resp.StatusCode,
			},
		},
	}, nil
}

// Enabled returns whether the Metaso client is configured with a valid API key.
func (c *MetasoClient) Enabled() bool {
	return c != nil && c.apiKey != ""
}

func normalizeMetasoSearchResult(query string, raw any, limit int) *SearchResult {
	if limit <= 0 {
		limit = 10
	}
	items := make([]SearchItem, 0, limit)
	seen := make(map[string]bool)
	collectMetasoSearchItems(raw, limit, seen, &items)
	return &SearchResult{
		Query:   query,
		Results: items,
		Answer:  firstStringDeep(raw, "answer", "summary", "generatedSummary"),
	}
}

func collectMetasoSearchItems(value any, limit int, seen map[string]bool, out *[]SearchItem) {
	if len(*out) >= limit {
		return
	}
	switch v := value.(type) {
	case []any:
		for _, entry := range v {
			if len(*out) >= limit {
				return
			}
			if m, ok := entry.(map[string]any); ok {
				if item, ok := metasoItemFromMap(m); ok {
					if !seen[item.URL] {
						seen[item.URL] = true
						*out = append(*out, item)
					}
					continue
				}
			}
			collectMetasoSearchItems(entry, limit, seen, out)
		}
	case map[string]any:
		if item, ok := metasoItemFromMap(v); ok {
			if !seen[item.URL] {
				seen[item.URL] = true
				*out = append(*out, item)
			}
			if len(*out) >= limit {
				return
			}
		}
		for _, key := range []string{"webpages", "webPages", "results", "items", "list", "docs", "documents", "data"} {
			if child, ok := v[key]; ok {
				collectMetasoSearchItems(child, limit, seen, out)
			}
			if len(*out) >= limit {
				return
			}
		}
	}
}

func metasoItemFromMap(m map[string]any) (SearchItem, bool) {
	url := firstString(m, "url", "link", "href", "webUrl", "web_url", "sourceUrl", "source_url")
	if url == "" {
		return SearchItem{}, false
	}
	title := firstString(m, "title", "name", "headline")
	if title == "" {
		title = url
	}
	content := firstString(m, "snippet", "summary", "content", "description", "text", "digest", "abstract")
	return SearchItem{
		Title:   title,
		URL:     url,
		Content: content,
		Score:   firstFloat(m, "score", "rankScore", "rank_score"),
	}, true
}

func normalizeMetasoReaderContent(body []byte) (content string, title string) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", ""
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return text, ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s), ""
	}
	content = firstStringDeep(raw, "markdown", "content", "text", "rawContent", "raw_content", "data")
	title = firstStringDeep(raw, "title", "name", "headline")
	if content == "" {
		return text, title
	}
	return content, title
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch typed := v.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case fmt.Stringer:
				if strings.TrimSpace(typed.String()) != "" {
					return strings.TrimSpace(typed.String())
				}
			}
		}
	}
	return ""
}

func firstStringDeep(value any, keys ...string) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		for _, entry := range v {
			if s := firstStringDeep(entry, keys...); s != "" {
				return s
			}
		}
	case map[string]any:
		if s := firstString(v, keys...); s != "" {
			return s
		}
		for _, key := range keys {
			if child, ok := v[key]; ok {
				if s := firstStringDeep(child, keys...); s != "" {
					return s
				}
			}
		}
		for _, child := range v {
			if s := firstStringDeep(child, keys...); s != "" {
				return s
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			switch typed := v.(type) {
			case float64:
				return typed
			case int:
				return float64(typed)
			case string:
				if f, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}
