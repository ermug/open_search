package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	//"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// PageData represents crawled page information
type PageData struct {
	URL           string            `json:"url"`
	StatusCode    int               `json:"status_code"`
	Content       string            `json:"content"`
	ContentType   string            `json:"content_type"`
	ContentLength int               `json:"content_length"`
	Headers       map[string]string `json:"headers"`
	FetchTime     time.Time         `json:"fetch_time"`
}

// IndexResult represents the result of indexing a page
type IndexResult struct {
	Success        bool    `json:"success"`
	ProcessingTime float64 `json:"processing_time"`
	WorkerID       int     `json:"worker_id"`
	Title          string  `json:"title"`
	WordCount      int     `json:"word_count"`
	ContentHash    string  `json:"content_hash"`
	Error          string  `json:"error,omitempty"`
}

// CrawlOutput combines page data and index result
type CrawlOutput struct {
	URL         string       `json:"url"`
	Timestamp   string       `json:"timestamp"`
	WorkerID    int          `json:"worker_id"`
	FilePath    string       `json:"file_path"` // Path to the raw HTML file
	Processed   bool         `json:"processed"` // Whether this file has been processed
	PageData    *PageData    `json:"page_data"`
	IndexResult *IndexResult `json:"index_result"`
}

// Stats tracks crawling statistics
type Stats struct {
	PagesCrawled int64                `json:"pages_crawled"`
	PagesIndexed int64                `json:"pages_indexed"`
	PagesFailed  int64                `json:"pages_failed"`
	StartTime    time.Time            `json:"start_time"`
	WorkerStats  map[int]*WorkerStats `json:"worker_stats"`
	RobotsCached int                  `json:"robots_cached"`
	mutex        sync.RWMutex
}

// WorkerStats tracks per-worker statistics
type WorkerStats struct {
	PagesCrawled        int64   `json:"pages_crawled"`
	PagesIndexed        int64   `json:"pages_indexed"`
	PagesFailed         int64   `json:"pages_failed"`
	TotalProcessingTime float64 `json:"total_processing_time"`
}

// URLFrontier manages the crawl queue in a thread-safe manner
type URLFrontier struct {
	queue      chan string
	visited    map[string]bool
	inProgress map[string]bool
	mutex      sync.RWMutex
}

// NewURLFrontier creates a new URL frontier
func NewURLFrontier(bufferSize int) *URLFrontier {
	return &URLFrontier{
		queue:      make(chan string, bufferSize),
		visited:    make(map[string]bool),
		inProgress: make(map[string]bool),
	}
}

// AddURL adds a URL to the frontier if not already seen
func (uf *URLFrontier) AddURL(url string) bool {
	uf.mutex.Lock()
	defer uf.mutex.Unlock()

	if uf.visited[url] || uf.inProgress[url] {
		return false
	}

	select {
	case uf.queue <- url:
		return true
	default:
		return false // Queue full
	}
}

// GetURL gets the next URL from the frontier
func (uf *URLFrontier) GetURL(ctx context.Context) (string, bool) {
	select {
	case url := <-uf.queue:
		uf.mutex.Lock()
		uf.inProgress[url] = true
		uf.mutex.Unlock()
		return url, true
	case <-ctx.Done():
		return "", false
	}
}

// MarkCompleted marks a URL as completed
func (uf *URLFrontier) MarkCompleted(url string) {
	uf.mutex.Lock()
	defer uf.mutex.Unlock()
	delete(uf.inProgress, url)
	uf.visited[url] = true
}

// MarkFailed marks a URL as failed
func (uf *URLFrontier) MarkFailed(url string) {
	uf.mutex.Lock()
	defer uf.mutex.Unlock()
	delete(uf.inProgress, url)
}

// Size returns the current queue size
func (uf *URLFrontier) Size() int {
	return len(uf.queue)
}

// VisitedCount returns the number of visited URLs
func (uf *URLFrontier) VisitedCount() int {
	uf.mutex.RLock()
	defer uf.mutex.RUnlock()
	return len(uf.visited)
}

// RobotsCache manages robots.txt caching
type RobotsCache struct {
	cache map[string]*RobotsRules
	mutex sync.RWMutex
}

// RobotsRules represents robots.txt rules
type RobotsRules struct {
	Allowed     bool
	CrawlDelay  time.Duration
	LastChecked time.Time
}

// NewRobotsCache creates a new robots cache
func NewRobotsCache() *RobotsCache {
	return &RobotsCache{
		cache: make(map[string]*RobotsRules),
	}
}

// GetRules gets robots.txt rules for a domain
func (rc *RobotsCache) GetRules(domain string, client *http.Client) *RobotsRules {
	rc.mutex.RLock()
	if rules, exists := rc.cache[domain]; exists {
		rc.mutex.RUnlock()
		return rules
	}
	rc.mutex.RUnlock()

	// Fetch robots.txt
	robotsURL := domain + "/robots.txt"
	rules := &RobotsRules{
		Allowed:     true, // Default to allowed
		CrawlDelay:  0,
		LastChecked: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err == nil {
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					rules = rc.parseRobotsTxt(string(body))
				}
			}
		}
	}

	rc.mutex.Lock()
	rc.cache[domain] = rules
	rc.mutex.Unlock()

	return rules
}

// parseRobotsTxt parses robots.txt content (simplified)
func (rc *RobotsCache) parseRobotsTxt(content string) *RobotsRules {
	rules := &RobotsRules{
		Allowed:     true,
		CrawlDelay:  0,
		LastChecked: time.Now(),
	}

	lines := strings.Split(content, "\n")
	var currentUserAgent string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch field {
		case "user-agent":
			currentUserAgent = value
		case "crawl-delay":
			if currentUserAgent == "*" {
				if delay, err := strconv.Atoi(value); err == nil {
					rules.CrawlDelay = time.Duration(delay) * time.Second
				}
			}
		case "disallow":
			if currentUserAgent == "*" && value == "/" {
				rules.Allowed = false
			}
		}
	}

	return rules
}

// Size returns the cache size
func (rc *RobotsCache) Size() int {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	return len(rc.cache)
}

// MockIndexer simulates the indexing process
type MockIndexer struct {
	WorkerID int
}

// NewMockIndexer creates a new mock indexer
func NewMockIndexer(workerID int) *MockIndexer {
	return &MockIndexer{WorkerID: workerID}
}

// IndexPage simulates indexing a page
func (mi *MockIndexer) IndexPage(pageData *PageData) *IndexResult {
	// Return minimal successful result without processing
	return &IndexResult{
		Success:        true,
		ProcessingTime: 0,
		WorkerID:       mi.WorkerID,
		Title:          "Not Indexed",
		WordCount:      0,
		ContentHash:    "disabled",
	}
}

// extractTitle extracts the title from HTML content
func extractTitle(content string) string {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "No Title"
	}

	var title string
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = strings.TrimSpace(n.FirstChild.Data)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
			if title != "" {
				return
			}
		}
	}

	findTitle(doc)
	if title == "" {
		return "No Title"
	}
	return title
}

// MultithreadedCrawler represents the main crawler
type MultithreadedCrawler struct {
	NumWorkers int
	Delay      time.Duration
	DumpDir    string
	MaxPages   int64

	urlFrontier *URLFrontier
	robotsCache *RobotsCache
	stats       *Stats
	client      *http.Client
	limiter     *rate.Limiter
	unprocessed *UnprocessedQueue

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMultithreadedCrawler creates a new crawler
func NewMultithreadedCrawler(numWorkers int, delay time.Duration, dumpDir string) *MultithreadedCrawler {
	ctx, cancel := context.WithCancel(context.Background())

	return &MultithreadedCrawler{
		NumWorkers:  numWorkers,
		Delay:       delay,
		DumpDir:     dumpDir,
		MaxPages:    100,
		urlFrontier: NewURLFrontier(10000000),
		robotsCache: NewRobotsCache(),
		stats: &Stats{
			WorkerStats: make(map[int]*WorkerStats),
			StartTime:   time.Now(),
		},
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: false,
			},
		},
		limiter:     rate.NewLimiter(rate.Every(delay), numWorkers),
		unprocessed: NewUnprocessedQueue(100, 0), // High watermark: 100, Low watermark: 0
		ctx:         ctx,
		cancel:      cancel,
	}
}

// normalizeURL normalizes a URL to avoid duplicates
func (mc *MultithreadedCrawler) normalizeURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	return parsed.String(), nil
}

// isValidURL checks if a URL is valid for crawling
func (mc *MultithreadedCrawler) isValidURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// extractLinks extracts links from HTML content
func (mc *MultithreadedCrawler) extractLinks(content string, baseURL string) []string {
	var links []string

	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return links
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return links
	}

	var extractLinksFromNode func(*html.Node)
	extractLinksFromNode = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					if resolved, err := base.Parse(attr.Val); err == nil {
						if mc.isValidURL(resolved.String()) {
							if normalized, err := mc.normalizeURL(resolved.String()); err == nil {
								links = append(links, normalized)
							}
						}
					}
					break
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractLinksFromNode(c)
		}
	}

	extractLinksFromNode(doc)

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueLinks := []string{}
	for _, link := range links {
		if !seen[link] {
			seen[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}

	return uniqueLinks
}

// fetchPage fetches a single page
func (mc *MultithreadedCrawler) fetchPage(rawURL string) (*PageData, error) {
	// Rate limiting
	if err := mc.limiter.Wait(mc.ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(mc.ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "MultithreadedCrawler/1.0 (Go)")

	resp, err := mc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &PageData{
		URL:           rawURL,
		StatusCode:    resp.StatusCode,
		Content:       string(body),
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: len(body),
		Headers:       headers,
		FetchTime:     time.Now(),
	}, nil
}

// savePageData saves crawled page data to file
func (mc *MultithreadedCrawler) savePageData(pageData *PageData, indexResult *IndexResult, workerID int) error {
	// Wait if we have too many unprocessed pages
	mc.unprocessed.WaitIfFull()

	if err := os.MkdirAll(mc.DumpDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("raw_%d_%s.json", workerID, timestamp)
	filepath := filepath.Join(mc.DumpDir, filename)

	// Save raw page data for later processing
	output := &CrawlOutput{
		URL:       pageData.URL,
		Timestamp: timestamp,
		WorkerID:  workerID,
		FilePath:  filepath,
		Processed: false,
		PageData:  pageData, // Save full page data
		IndexResult: &IndexResult{
			Success:     true,
			WorkerID:    workerID,
			ContentHash: "pending",
		},
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return err
	}

	// Increment unprocessed count after successful save
	mc.unprocessed.Increment()

	return nil
}

// updateStats updates crawler statistics
func (mc *MultithreadedCrawler) updateStats(workerID int, action string, processingTime float64) {
	mc.stats.mutex.Lock()
	defer mc.stats.mutex.Unlock()

	if mc.stats.WorkerStats[workerID] == nil {
		mc.stats.WorkerStats[workerID] = &WorkerStats{}
	}

	switch action {
	case "crawled":
		mc.stats.PagesCrawled++
		mc.stats.WorkerStats[workerID].PagesCrawled++
	case "indexed":
		mc.stats.PagesIndexed++
		mc.stats.WorkerStats[workerID].PagesIndexed++
		mc.stats.WorkerStats[workerID].TotalProcessingTime += processingTime
	case "failed":
		mc.stats.PagesFailed++
		mc.stats.WorkerStats[workerID].PagesFailed++
	}
}

// printStats prints current crawling statistics
func (mc *MultithreadedCrawler) printStats() {
	mc.stats.mutex.RLock()
	defer mc.stats.mutex.RUnlock()

	totalTime := time.Since(mc.stats.StartTime).Seconds()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CRAWLING STATISTICS")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total Pages Crawled: %d\n", mc.stats.PagesCrawled)
	fmt.Printf("Total Pages Indexed: %d\n", mc.stats.PagesIndexed)
	fmt.Printf("Total Pages Failed: %d\n", mc.stats.PagesFailed)
	fmt.Printf("URLs in Frontier: %d\n", mc.urlFrontier.Size())
	fmt.Printf("URLs Visited: %d\n", mc.urlFrontier.VisitedCount())
	fmt.Printf("Unprocessed Pages: %d\n", mc.unprocessed.GetCount())
	fmt.Printf("Total Runtime: %.2fs\n", totalTime)

	if mc.stats.PagesCrawled > 0 {
		pagesPerSecond := float64(mc.stats.PagesCrawled) / totalTime
		fmt.Printf("Pages per Second: %.2f\n", pagesPerSecond)
	}

	fmt.Printf("Active Workers: %d\n", mc.NumWorkers)
	fmt.Printf("Robots.txt Cached: %d\n", mc.robotsCache.Size())

	fmt.Println("\nPER-WORKER STATISTICS:")
	for workerID, stats := range mc.stats.WorkerStats {
		avgProcessing := stats.TotalProcessingTime / float64(max(stats.PagesIndexed, 1))
		fmt.Printf("  Worker-%d: %d crawled, %d indexed, %d failed, avg processing: %.2fs\n",
			workerID, stats.PagesCrawled, stats.PagesIndexed, stats.PagesFailed, avgProcessing)
	}
}

// max returns the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// UnprocessedQueue manages the queue of unprocessed pages
type UnprocessedQueue struct {
	count         int64
	mutex         sync.RWMutex
	highWatermark int64
	lowWatermark  int64
	pauseCond     *sync.Cond
}

// NewUnprocessedQueue creates a new unprocessed queue manager
func NewUnprocessedQueue(highWatermark, lowWatermark int64) *UnprocessedQueue {
	uq := &UnprocessedQueue{
		highWatermark: highWatermark,
		lowWatermark:  lowWatermark,
	}
	uq.pauseCond = sync.NewCond(&uq.mutex)
	return uq
}

// Increment increases the count of unprocessed pages
func (uq *UnprocessedQueue) Increment() {
	uq.mutex.Lock()
	uq.count++
	uq.mutex.Unlock()
}

// Decrement decreases the count of unprocessed pages
func (uq *UnprocessedQueue) Decrement() {
	uq.mutex.Lock()
	if uq.count > 0 {
		uq.count--
		// If we've gone below lowWatermark, signal crawlers to resume
		if uq.count <= uq.lowWatermark {
			uq.pauseCond.Broadcast()
		}
	}
	uq.mutex.Unlock()
}

// WaitIfFull blocks if the queue is full until it's below lowWatermark
func (uq *UnprocessedQueue) WaitIfFull() {
	uq.mutex.Lock()
	for uq.count >= uq.highWatermark {
		uq.pauseCond.Wait()
	}
	uq.mutex.Unlock()
}

// GetCount returns the current count of unprocessed pages
func (uq *UnprocessedQueue) GetCount() int64 {
	uq.mutex.RLock()
	defer uq.mutex.RUnlock()
	return uq.count
}

// DeleteProcessedFile removes a file after it has been processed and updates the unprocessed count
func (mc *MultithreadedCrawler) DeleteProcessedFile(filepath string) error {
	// First verify the file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", filepath)
	}

	// Delete the file
	if err := os.Remove(filepath); err != nil {
		return fmt.Errorf("failed to delete file %s: %v", filepath, err)
	}

	// Decrement the unprocessed count
	mc.unprocessed.Decrement()

	return nil
}

// crawlerWorker represents an individual crawler worker
func (mc *MultithreadedCrawler) crawlerWorker(workerID int) {
	defer mc.wg.Done()

	log.Printf("[WORKER-%d] Starting crawler worker", workerID)
	indexer := NewMockIndexer(workerID)

	for {
		select {
		case <-mc.ctx.Done():
			log.Printf("[WORKER-%d] Received shutdown signal", workerID)
			return
		default:
		}

		// Check if we've reached the maximum pages
		mc.stats.mutex.RLock()
		if mc.stats.PagesCrawled >= mc.MaxPages {
			mc.stats.mutex.RUnlock()
			log.Printf("[WORKER-%d] Maximum pages reached", workerID)
			return
		}
		mc.stats.mutex.RUnlock()

		// Get next URL
		urlStr, ok := mc.urlFrontier.GetURL(mc.ctx)
		if !ok {
			log.Printf("[WORKER-%d] No more URLs or context cancelled", workerID)
			return
		}

		log.Printf("[WORKER-%d] Processing: %s", workerID, urlStr)

		// Parse URL to get domain
		parsed, err := url.Parse(urlStr)
		if err != nil {
			log.Printf("[WORKER-%d] Invalid URL %s: %v", workerID, urlStr, err)
			mc.urlFrontier.MarkFailed(urlStr)
			mc.updateStats(workerID, "failed", 0)
			continue
		}

		domain := parsed.Scheme + "://" + parsed.Host

		// Check robots.txt
		rules := mc.robotsCache.GetRules(domain, mc.client)
		if !rules.Allowed {
			log.Printf("[WORKER-%d] Skipping %s due to robots.txt", workerID, urlStr)
			mc.urlFrontier.MarkFailed(urlStr)
			continue
		}

		// Apply robots.txt delay
		if rules.CrawlDelay > 0 {
			time.Sleep(rules.CrawlDelay)
		}

		// Fetch page
		pageData, err := mc.fetchPage(urlStr)
		if err != nil {
			log.Printf("[WORKER-%d] Failed to fetch %s: %v", workerID, urlStr, err)
			mc.urlFrontier.MarkFailed(urlStr)
			mc.updateStats(workerID, "failed", 0)
			continue
		}

		mc.updateStats(workerID, "crawled", 0)

		// Index page
		indexResult := indexer.IndexPage(pageData)

		if indexResult.Success {
			mc.updateStats(workerID, "indexed", indexResult.ProcessingTime)
			log.Printf("[WORKER-%d] Successfully indexed: %s", workerID, indexResult.Title[:min(50, len(indexResult.Title))])

			// Save page data
			if err := mc.savePageData(pageData, indexResult, workerID); err != nil {
				log.Printf("[WORKER-%d] Failed to save page data: %v", workerID, err)
			}

			// Extract and add new links
			links := mc.extractLinks(pageData.Content, urlStr)
			newLinks := 0
			for _, link := range links {
				if mc.urlFrontier.AddURL(link) {
					newLinks++
				}
			}

			log.Printf("[WORKER-%d] Added %d new links to frontier", workerID, newLinks)
		} else {
			log.Printf("[WORKER-%d] Indexing failed: %s", workerID, indexResult.Error)
			mc.updateStats(workerID, "failed", 0)
		}

		// Mark as completed
		mc.urlFrontier.MarkCompleted(urlStr)

		// Apply politeness delay
		time.Sleep(mc.Delay)
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Crawl starts the multithreaded crawling process
func (mc *MultithreadedCrawler) Crawl(seedURLs []string, maxPages int64, politenessDelay time.Duration) (int64, error) {
	if politenessDelay > 0 {
		mc.Delay = politenessDelay
	}

	mc.MaxPages = maxPages
	mc.stats.StartTime = time.Now()

	// Add seed URLs
	for _, rawURL := range seedURLs {
		if normalized, err := mc.normalizeURL(rawURL); err == nil {
			mc.urlFrontier.AddURL(normalized)
		}
	}

	fmt.Printf("Starting multithreaded crawl with %d workers\n", mc.NumWorkers)
	fmt.Printf("Max pages: %d, Politeness delay: %v\n", maxPages, mc.Delay)
	fmt.Printf("Seed URLs: %d\n", len(seedURLs))
	fmt.Printf("Output directory: %s\n", mc.DumpDir)
	fmt.Println(strings.Repeat("=", 60))

	// Start worker goroutines
	for i := 0; i < mc.NumWorkers; i++ {
		mc.wg.Add(1)
		go mc.crawlerWorker(i)
	}

	// Monitor progress
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mc.printStats()

				// Check stopping conditions
				mc.stats.mutex.RLock()
				shouldStop := mc.stats.PagesCrawled >= mc.MaxPages ||
					(mc.urlFrontier.Size() == 0 && mc.urlFrontier.VisitedCount() > 0)
				mc.stats.mutex.RUnlock()

				if shouldStop {
					log.Println("Stopping crawl...")
					mc.cancel()
					return
				}
			case <-mc.ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to complete
	mc.wg.Wait()

	// Final statistics
	mc.printStats()
	fmt.Printf("\nCrawl completed! Check %s for results.\n", mc.DumpDir)

	return mc.stats.PagesCrawled, nil
}

func main() {
	fmt.Println("Multithreaded Web Crawler and Indexer in Go")
	fmt.Println(strings.Repeat("=", 50))

	// Initialize crawler
	crawler := NewMultithreadedCrawler(
		80,             // num workers
		1*time.Second,  // delay
		"crawl_output", // dump directory
	)

	// Initialize indexer
	indexer, err := NewTestIndexer("index_output")
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	// Start indexer goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check crawl_output directory for files
				files, err := os.ReadDir(crawler.DumpDir)
				if err != nil {
					log.Printf("Error reading directory: %v", err)
					continue
				}

				for _, file := range files {
					if !file.IsDir() && strings.HasPrefix(file.Name(), "raw_") {
						filepath := filepath.Join(crawler.DumpDir, file.Name())
						if err := indexer.ProcessAndDeleteFile(filepath, crawler); err != nil {
							log.Printf("Error processing file %s: %v", file.Name(), err)
							continue
						}
						log.Printf("Successfully processed and indexed: %s", file.Name())
					}
				}

				// Print indexer stats
				stats := indexer.GetStats()
				log.Printf("Indexer stats: Terms: %d, Documents: %d",
					stats["total_terms"], stats["total_documents"])
			}
		}
	}()

	// Seed URLs for crawling
	seedURLs := []string{
		"https://en.wikipedia.org/wiki/Web_crawler",
		"https://en.wikipedia.org/wiki/World_War_II",
		"https://en.wikipedia.org/wiki/Halogen",
		"https://en.wikipedia.org/wiki/Taylor_Swift",
		"https://en.wikipedia.org/wiki/Heartburn",
		"https://en.wikipedia.org/wiki/Michael_Jackson",
		"https://en.wikipedia.org/wiki/Michelangelo",
		"https://en.wikipedia.org/wiki/Middle_East",
		"https://en.wikipedia.org/wiki/Astronomy",
		"https://www.reddit.com",
		"https://store.steampowered.com/app/1262350/SIGNALIS/",
	}

	// Start crawling
	pagesCrawled, err := crawler.Crawl(
		seedURLs,
		12000,         // max pages
		2*time.Second, // politeness delay
	)

	if err != nil {
		log.Fatalf("Error during crawling: %v", err)
	}

	fmt.Printf("\nCrawling completed! Total pages processed: %d\n", pagesCrawled)

	// Final index save
	if err := indexer.SaveIndex(); err != nil {
		log.Printf("Error saving final index: %v", err)
	}
}
