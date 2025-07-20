package main

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Common English stop words
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
	"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
	"that": true, "the": true, "to": true, "was": true, "were": true, "will": true,
	"with": true, "this": true, "but": true, "they": true, "have": true,
	"had": true, "what": true, "when": true, "where": true, "who": true, "which": true,
	"why": true, "how": true, "all": true, "any": true, "both": true, "each": true,
	"few": true, "more": true, "most": true, "other": true, "some": true, "such": true,
	"no": true, "nor": true, "not": true, "only": true, "own": true, "same": true,
	"so": true, "than": true, "too": true, "very": true, "can": true, "did": true,
	"do": true, "does": true, "doing": true, "done": true, "down": true, "up": true,
}

// TextProcessor handles text processing operations
type TextProcessor struct {
	stripTagsRegex *regexp.Regexp
	stripHTMLRegex *regexp.Regexp
}

// NewTextProcessor creates a new text processor
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{
		stripTagsRegex: regexp.MustCompile(`<[^>]*>`),
		stripHTMLRegex: regexp.MustCompile(`&[a-zA-Z]+;`),
	}
}

// CleanHTML removes scripts, styles, and extracts clean text from HTML
func (tp *TextProcessor) CleanHTML(content string) (string, string) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", ""
	}

	var title string
	var text strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip script and style elements
			if n.Data == "script" || n.Data == "style" || n.Data == "noscript" {
				return
			}
			// Extract title
			if n.Data == "title" && n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
			}
		}
		if n.Type == html.TextNode {
			text.WriteString(strings.TrimSpace(n.Data))
			text.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	// Clean up HTML entities and extra whitespace
	cleanText := tp.stripHTMLRegex.ReplaceAllString(text.String(), " ")
	cleanText = strings.Join(strings.Fields(cleanText), " ")

	return title, cleanText
}

// ProcessText cleans and normalizes text
func (tp *TextProcessor) ProcessText(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split into words
	words := strings.Fields(text)

	// Process each word
	var result []string
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,!?()[]{}:;\"'")

		// Skip if empty or stop word
		if word == "" || stopWords[word] {
			continue
		}

		// Apply stemming
		word = tp.stem(word)

		result = append(result, word)
	}

	return result
}

// stem implements a simple Porter Stemmer algorithm
func (tp *TextProcessor) stem(word string) string {
	// Skip short words
	if len(word) < 3 {
		return word
	}

	// Step 1a: Remove plurals and -ed/-ing
	if strings.HasSuffix(word, "sses") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "ies") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "ss") {
		// Do nothing
	} else if strings.HasSuffix(word, "s") {
		word = word[:len(word)-1]
	}

	// Step 1b: Remove -ed and -ing
	if strings.HasSuffix(word, "eed") {
		word = word[:len(word)-1]
	} else if strings.HasSuffix(word, "ed") {
		word = word[:len(word)-2]
	} else if strings.HasSuffix(word, "ing") {
		word = word[:len(word)-3]
	}

	// Step 2: Remove -ization, -ational, etc.
	if strings.HasSuffix(word, "ization") {
		word = word[:len(word)-5] + "ize"
	} else if strings.HasSuffix(word, "ational") {
		word = word[:len(word)-5] + "ate"
	} else if strings.HasSuffix(word, "fulness") {
		word = word[:len(word)-4]
	} else if strings.HasSuffix(word, "ousness") {
		word = word[:len(word)-4]
	} else if strings.HasSuffix(word, "iveness") {
		word = word[:len(word)-4]
	}

	// Step 3: Remove -ize, -ate, etc.
	if strings.HasSuffix(word, "ize") {
		word = word[:len(word)-3]
	} else if strings.HasSuffix(word, "ate") {
		word = word[:len(word)-3]
	} else if strings.HasSuffix(word, "al") {
		word = word[:len(word)-2]
	}

	return word
}
