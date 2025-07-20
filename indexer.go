package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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

// TestIndexer represents a simple test indexer
type TestIndexer struct {
	indexDir      string
	terms         map[string]*TermEntry
	documents     map[string]*IndexedDocument
	termLock      sync.RWMutex
	documentLock  sync.RWMutex
	textProcessor *TextProcessor
}

// NewTestIndexer creates a new test indexer
func NewTestIndexer(indexDir string) (*TestIndexer, error) {
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, err
	}

	return &TestIndexer{
		indexDir:      indexDir,
		terms:         make(map[string]*TermEntry),
		documents:     make(map[string]*IndexedDocument),
		textProcessor: NewTextProcessor(),
	}, nil
}

// ProcessDocument processes a single document and updates the index
func (ti *TestIndexer) ProcessDocument(output *CrawlOutput) error {
	// Clean HTML and extract text
	title, cleanText := ti.textProcessor.CleanHTML(output.PageData.Content)
	if title == "" {
		title = "No Title"
	}

	// Process text into terms
	words := ti.textProcessor.ProcessText(cleanText)

	// Create document entry
	docID := fmt.Sprintf("doc_%d", len(ti.documents)+1)
	document := &IndexedDocument{
		ID:        docID,
		URL:       output.URL,
		Title:     title,
		Length:    len(words),
		Timestamp: time.Now(),
	}

	// Update document store
	ti.documentLock.Lock()
	ti.documents[docID] = document
	ti.documentLock.Unlock()

	// Process terms and update inverted index
	termPositions := make(map[string][]int)
	for pos, word := range words {
		termPositions[word] = append(termPositions[word], pos)
	}

	// Update term dictionary
	ti.termLock.Lock()
	for term, positions := range termPositions {
		if ti.terms[term] == nil {
			ti.terms[term] = &TermEntry{
				Term:      term,
				Documents: make(map[string]Occurrence),
			}
		}

		ti.terms[term].Documents[docID] = Occurrence{
			Frequency: len(positions),
			Positions: positions,
		}
		ti.terms[term].DocCount = len(ti.terms[term].Documents)
	}
	ti.termLock.Unlock()

	return nil
}

// SaveIndex saves the current index to disk
func (ti *TestIndexer) SaveIndex() error {
	// Save terms
	termsFile := filepath.Join(ti.indexDir, "terms.json")
	ti.termLock.RLock()
	termsData, err := json.MarshalIndent(ti.terms, "", "  ")
	ti.termLock.RUnlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(termsFile, termsData, 0644); err != nil {
		return err
	}

	// Save documents
	docsFile := filepath.Join(ti.indexDir, "docs.json")
	ti.documentLock.RLock()
	docsData, err := json.MarshalIndent(ti.documents, "", "  ")
	ti.documentLock.RUnlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(docsFile, docsData, 0644); err != nil {
		return err
	}

	return nil
}

// Search performs a simple search on the index
func (ti *TestIndexer) Search(query string) []string {
	// Process query terms
	queryTerms := ti.textProcessor.ProcessText(query)

	ti.termLock.RLock()
	defer ti.termLock.RUnlock()

	results := make(map[string]int)

	// Simple TF scoring
	for _, term := range queryTerms {
		if entry, exists := ti.terms[term]; exists {
			for docID, occ := range entry.Documents {
				results[docID] += occ.Frequency
			}
		}
	}

	// Convert results to sorted list
	var urls []string
	ti.documentLock.RLock()
	for docID := range results {
		if doc, exists := ti.documents[docID]; exists {
			urls = append(urls, doc.URL)
		}
	}
	ti.documentLock.RUnlock()

	return urls
}

// GetStats returns indexing statistics
func (ti *TestIndexer) GetStats() map[string]interface{} {
	ti.termLock.RLock()
	ti.documentLock.RLock()
	defer ti.termLock.RUnlock()
	defer ti.documentLock.RUnlock()

	return map[string]interface{}{
		"total_terms":     len(ti.terms),
		"total_documents": len(ti.documents),
	}
}

// ProcessAndDeleteFile processes a file and deletes it after successful processing
func (ti *TestIndexer) ProcessAndDeleteFile(filepath string, crawler *MultithreadedCrawler) error {
	// Read the file
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Parse the CrawlOutput
	var output CrawlOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return fmt.Errorf("failed to parse file: %v", err)
	}

	// Process the document
	if err := ti.ProcessDocument(&output); err != nil {
		return fmt.Errorf("failed to process document: %v", err)
	}

	// Save index periodically (every 10 documents)
	ti.documentLock.RLock()
	docCount := len(ti.documents)
	ti.documentLock.RUnlock()
	if docCount%10 == 0 {
		if err := ti.SaveIndex(); err != nil {
			log.Printf("Warning: Failed to save index: %v", err)
		}
	}

	// Delete the processed file
	if err := crawler.DeleteProcessedFile(filepath); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	return nil
}
