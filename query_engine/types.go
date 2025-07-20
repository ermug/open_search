package query_engine

import "time"

// TermEntry represents a term in the inverted index
type TermEntry struct {
	Term      string                `json:"term"`
	DocCount  int                   `json:"doc_count"`
	Documents map[string]Occurrence `json:"documents"`
}

// Occurrence represents a term's occurrence in a document
type Occurrence struct {
	Frequency int   `json:"frequency"`
	Positions []int `json:"positions"`
}

// IndexedDocument represents metadata about an indexed document
type IndexedDocument struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

// SearchResult represents a single search result
type SearchResult struct {
	URL      string  `json:"url"`
	Title    string  `json:"title"`
	Score    float64 `json:"score"`
	Snippet  string  `json:"snippet"`
	DocID    string  `json:"doc_id"`
	DocScore int     `json:"doc_score"`
}
