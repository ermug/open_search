package query_engine

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// QueryEngine represents the search engine
type QueryEngine struct {
	terms     map[string]*TermEntry
	documents map[string]*IndexedDocument
	indexDir  string
}

// NewQueryEngine creates a new query engine instance
func NewQueryEngine(indexDir string) (*QueryEngine, error) {
	qe := &QueryEngine{
		indexDir: indexDir,
	}

	// Load terms index
	termsFile := filepath.Join(indexDir, "terms.json")
	termsData, err := os.ReadFile(termsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read terms index: %v", err)
	}

	if err := json.Unmarshal(termsData, &qe.terms); err != nil {
		return nil, fmt.Errorf("failed to parse terms index: %v", err)
	}

	// Load documents index
	docsFile := filepath.Join(indexDir, "docs.json")
	docsData, err := os.ReadFile(docsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read documents index: %v", err)
	}

	if err := json.Unmarshal(docsData, &qe.documents); err != nil {
		return nil, fmt.Errorf("failed to parse documents index: %v", err)
	}

	return qe, nil
}

// Search performs a search query and returns ranked results
func (qe *QueryEngine) Search(query string, maxResults int) ([]SearchResult, error) {
	// Process query terms
	queryTerms := strings.Fields(strings.ToLower(query))

	// Calculate document scores using TF-IDF
	scores := make(map[string]float64)
	docFreq := make(map[string]int)

	// Calculate document frequencies
	totalDocs := float64(len(qe.documents))
	for _, term := range queryTerms {
		if entry, exists := qe.terms[term]; exists {
			docFreq[term] = entry.DocCount
		}
	}

	// Calculate scores for each document
	for _, term := range queryTerms {
		if entry, exists := qe.terms[term]; exists {
			// IDF = log(N/df)
			idf := math.Log(totalDocs / float64(docFreq[term]))

			for docID, occ := range entry.Documents {
				// TF = frequency of term in document
				tf := float64(occ.Frequency)

				// Basic position boost: terms appearing earlier get higher scores
				positionBoost := 1.0
				if len(occ.Positions) > 0 && occ.Positions[0] < 100 {
					positionBoost = 1.2
				}

				// Title boost: terms appearing in title get higher scores
				titleBoost := 1.0
				if doc, exists := qe.documents[docID]; exists {
					if strings.Contains(strings.ToLower(doc.Title), term) {
						titleBoost = 1.5
					}
				}

				// Final score for this term in this document
				score := tf * idf * positionBoost * titleBoost
				scores[docID] += score
			}
		}
	}

	// Convert scores to results
	var results []SearchResult
	for docID, score := range scores {
		if doc, exists := qe.documents[docID]; exists {
			result := SearchResult{
				URL:      doc.URL,
				Title:    doc.Title,
				Score:    score,
				DocID:    docID,
				DocScore: int(score * 1000), // Scaled score for display
			}
			results = append(results, result)
		}
	}

	// Sort results by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// GetSuggestions returns related search terms
func (qe *QueryEngine) GetSuggestions(query string, maxSuggestions int) []string {
	queryTerms := strings.Fields(strings.ToLower(query))
	if len(queryTerms) == 0 {
		return nil
	}

	// Find documents matching the query
	matchingDocs := make(map[string]bool)
	for _, term := range queryTerms {
		if entry, exists := qe.terms[term]; exists {
			for docID := range entry.Documents {
				matchingDocs[docID] = true
			}
		}
	}

	// Count term frequencies in matching documents
	termFreq := make(map[string]int)
	for docID := range matchingDocs {
		for term, entry := range qe.terms {
			if _, exists := entry.Documents[docID]; exists {
				termFreq[term]++
			}
		}
	}

	// Convert to slice for sorting
	type termScore struct {
		term  string
		score int
	}
	var suggestions []termScore
	for term, freq := range termFreq {
		// Skip original query terms
		isQueryTerm := false
		for _, queryTerm := range queryTerms {
			if term == queryTerm {
				isQueryTerm = true
				break
			}
		}
		if !isQueryTerm {
			suggestions = append(suggestions, termScore{term, freq})
		}
	}

	// Sort by frequency
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].score > suggestions[j].score
	})

	// Convert to string slice
	var result []string
	for i := 0; i < len(suggestions) && i < maxSuggestions; i++ {
		result = append(result, suggestions[i].term)
	}

	return result
}

// GetStats returns index statistics
func (qe *QueryEngine) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"total_terms":     len(qe.terms),
		"total_documents": len(qe.documents),
	}
}
