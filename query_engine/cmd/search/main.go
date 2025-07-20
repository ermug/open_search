package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"open_search/query_engine"
)

func main() {
	// Initialize query engine
	qe, err := query_engine.NewQueryEngine("../../../index_output")
	if err != nil {
		log.Fatalf("Failed to initialize query engine: %v", err)
	}

	// Print stats
	stats := qe.GetStats()
	fmt.Printf("Search Engine Ready!\n")
	fmt.Printf("Index contains %d terms across %d documents\n",
		stats["total_terms"], stats["total_documents"])
	fmt.Println("\nEnter your search query (or 'quit' to exit):")

	// Start REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		query := strings.TrimSpace(scanner.Text())
		if query == "" {
			continue
		}
		if query == "quit" || query == "exit" {
			break
		}

		// Perform search
		results, err := qe.Search(query, 5)
		if err != nil {
			fmt.Printf("Search error: %v\n", err)
			continue
		}

		// Print results
		if len(results) == 0 {
			fmt.Println("No results found.")
		} else {
			fmt.Printf("\nFound %d results:\n", len(results))
			fmt.Println(strings.Repeat("-", 60))

			for i, result := range results {
				fmt.Printf("%d. %s\n", i+1, result.Title)
				fmt.Printf("   URL: %s\n", result.URL)
				fmt.Printf("   Score: %.4f\n", result.Score)
				if result.Snippet != "" {
					fmt.Printf("   Snippet: %s\n", result.Snippet)
				}
				fmt.Println()
			}
		}

		// Get and print suggestions
		suggestions := qe.GetSuggestions(query, 5)
		if len(suggestions) > 0 {
			fmt.Println("Related terms:")
			for i, term := range suggestions {
				fmt.Printf("- %s", term)
				if i < len(suggestions)-1 {
					fmt.Print(", ")
				}
			}
			fmt.Println("\n")
		}
	}

	fmt.Println("\nGoodbye!")
}
