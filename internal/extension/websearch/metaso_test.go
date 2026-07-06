package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetasoSearchShapesRequestAndNormalizesResults(t *testing.T) {
	var seenPath string
	var seenAuth string
	var seenPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"webpages": [
					{"title": "Result A", "url": "https://example.com/a", "snippet": "Alpha", "score": 0.91}
				]
			}
		}`))
	}))
	defer server.Close()

	client := NewMetasoClient("mk-test")
	client.baseURL = server.URL
	result, err := client.Search(context.Background(), SearchRequest{Query: "game market", MaxResults: 7})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if seenPath != "/search" {
		t.Fatalf("path=%q, want /search", seenPath)
	}
	if seenAuth != "Bearer mk-test" {
		t.Fatalf("Authorization=%q", seenAuth)
	}
	if seenPayload["q"] != "game market" || seenPayload["scope"] != "webpage" || seenPayload["size"] != "7" {
		t.Fatalf("payload=%+v", seenPayload)
	}
	if len(result.Results) != 1 {
		t.Fatalf("results=%d, want 1", len(result.Results))
	}
	item := result.Results[0]
	if item.Title != "Result A" || item.URL != "https://example.com/a" || item.Content != "Alpha" {
		t.Fatalf("item=%+v", item)
	}
}

func TestMetasoFetchReadsPlainText(t *testing.T) {
	var seenPath string
	var seenAuth string
	var seenPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&seenPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Readable page content"))
	}))
	defer server.Close()

	client := NewMetasoClient("mk-test")
	client.baseURL = server.URL
	result, err := client.Fetch(context.Background(), FetchRequest{URL: "https://example.com/news"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if seenPath != "/reader" {
		t.Fatalf("path=%q, want /reader", seenPath)
	}
	if seenAuth != "Bearer mk-test" {
		t.Fatalf("Authorization=%q", seenAuth)
	}
	if seenPayload["url"] != "https://example.com/news" {
		t.Fatalf("payload=%+v", seenPayload)
	}
	if result.Data.Markdown != "Readable page content" {
		t.Fatalf("Markdown=%q", result.Data.Markdown)
	}
	if result.Data.Metadata.SourceURL != "https://example.com/news" {
		t.Fatalf("SourceURL=%q", result.Data.Metadata.SourceURL)
	}
}
