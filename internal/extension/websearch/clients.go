package websearch

// NewSearchClient returns the preferred search client for injected web search.
// Metaso takes precedence because it supplies both search and reader APIs.
func NewSearchClient(tavilyKey, metasoKey string) SearchClient {
	if metasoKey != "" {
		return NewMetasoClient(metasoKey)
	}
	if tavilyKey != "" {
		return NewTavilyClient(tavilyKey)
	}
	return nil
}

// NewFetchClient returns the preferred page reader client for injected web search.
// Metaso Reader takes precedence over Firecrawl when configured.
func NewFetchClient(metasoKey, firecrawlKey string) FetchClient {
	if metasoKey != "" {
		return NewMetasoClient(metasoKey)
	}
	if firecrawlKey != "" {
		return NewFirecrawlClient(firecrawlKey)
	}
	return nil
}
