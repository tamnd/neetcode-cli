// Package neetcode is the library behind the nc command: the HTTP client,
// request shaping, and the typed data models for NeetCode.
//
// NeetCode (neetcode.io) embeds its full problem list in the Angular app
// bundle as a JavaScript array. This client fetches that bundle, extracts the
// array with a small regex/string parse, and returns typed Problem records —
// no API key, no authentication, completely open.
package neetcode

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to neetcode. A real, honest
// User-Agent is both polite and the thing most likely to keep you unblocked.
const DefaultUserAgent = "nc/dev (+https://github.com/tamnd/neetcode-cli)"

const (
	defaultBaseURL  = "https://neetcode.io"
	leetcodeBase    = "https://leetcode.com/problems/"
	youtubeEmbedBase = "https://youtube.com/embed/"
	neetcodeBase    = "https://neetcode.io/problems/"
)

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   defaultBaseURL,
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to neetcode over HTTP.
type Client struct {
	http      *http.Client
	baseURL   string
	userAgent string
	rate      time.Duration
	retries   int
	last      time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		http:      &http.Client{Timeout: cfg.Timeout},
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		userAgent: cfg.UserAgent,
		rate:      cfg.Rate,
		retries:   cfg.Retries,
	}
}

// Problem is the canonical record for a NeetCode problem.
type Problem struct {
	Title        string `json:"title"`
	Difficulty   string `json:"difficulty"`
	Category     string `json:"category"`
	VideoURL     string `json:"video_url"`
	LeetcodeURL  string `json:"leetcode_url"`
	NeetcodeURL  string `json:"neetcode_url"`
	IsNeetcode150 bool  `json:"neetcode_150"`
	IsBlind75     bool  `json:"blind_75"`
	IsNeetcode250 bool  `json:"neetcode_250"`
}

// Problems fetches the full problem list and returns up to limit records.
// If limit <= 0, all problems are returned.
func (c *Client) Problems(ctx context.Context, limit int) ([]Problem, error) {
	all, err := c.fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	if limit > 0 && limit < len(all) {
		return all[:limit], nil
	}
	return all, nil
}

// Search returns problems whose title or category contains query
// (case-insensitive) up to limit records. If limit <= 0, all matches returned.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Problem, error) {
	all, err := c.fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Problem
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Title), q) ||
			strings.Contains(strings.ToLower(p.Category), q) {
			out = append(out, p)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// fetchAll downloads the main Angular bundle and parses the embedded problem list.
func (c *Client) fetchAll(ctx context.Context) ([]Problem, error) {
	// Step 1: fetch the index page to find the current bundle filename.
	indexBody, err := c.get(ctx, c.baseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("fetch index: %w", err)
	}
	bundleURL, err := extractBundleURL(c.baseURL, indexBody)
	if err != nil {
		return nil, err
	}

	// Step 2: fetch the bundle and extract the problem array.
	bundleBody, err := c.get(ctx, bundleURL)
	if err != nil {
		return nil, fmt.Errorf("fetch bundle: %w", err)
	}
	return parseBundle(bundleBody)
}

// mainBundleRe matches the main Angular chunk URL in the HTML page.
var mainBundleRe = regexp.MustCompile(`src="(main\.[a-f0-9]+\.js)"`)

func extractBundleURL(baseURL string, body []byte) (string, error) {
	m := mainBundleRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("could not find main bundle script tag in page HTML")
	}
	return baseURL + "/" + string(m[1]), nil
}

// parseBundle finds Q=[{problem:...},...] in the JS bundle and turns it into Problems.
func parseBundle(body []byte) ([]Problem, error) {
	s := string(body)
	// Locate the problem array: looks like Q=[{problem:"...",...},{...}]
	marker := "Q=[{problem:"
	idx := strings.Index(s, marker)
	if idx < 0 {
		return nil, fmt.Errorf("problem array not found in bundle (site may have changed)")
	}
	// Walk from the [ to find the matching ]
	start := idx + len("Q=")
	depth := 0
	end := -1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("unterminated problem array in bundle")
	}
	return parseJSArray(s[start:end])
}

// parseJSArray converts a JS object-literal array to []Problem without a full JS parser.
// It splits on },{, parses each object's key:value pairs manually.
func parseJSArray(src string) ([]Problem, error) {
	// Strip outer [ and ]
	src = strings.TrimSpace(src)
	if len(src) < 2 {
		return nil, fmt.Errorf("empty array")
	}
	src = src[1 : len(src)-1] // strip []

	// Split on object boundaries. Each object is {k:v,...}
	objects := splitObjects(src)
	out := make([]Problem, 0, len(objects))
	for _, obj := range objects {
		p, err := parseJSObject(obj)
		if err != nil {
			continue // skip malformed entries
		}
		out = append(out, p)
	}
	return out, nil
}

// splitObjects splits a comma-delimited sequence of {…} JS objects.
// It respects brace nesting so nested strings with commas don't split wrong.
func splitObjects(src string) []string {
	var objects []string
	depth := 0
	start := -1
	for i, c := range src {
		switch c {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				objects = append(objects, src[start:i+1])
				start = -1
			}
		}
	}
	return objects
}

// jsKeyValueRe matches key:"value" or key:!0 or key:!1 inside a JS object literal.
var jsKeyValueRe = regexp.MustCompile(`(\w+):"([^"]*)"`)
var jsBoolRe = regexp.MustCompile(`(\w+):(!0|!1|true|false)`)

// parseJSObject extracts known fields from a single JS object literal string.
func parseJSObject(obj string) (Problem, error) {
	// Strip outer braces
	obj = strings.TrimSpace(obj)
	if len(obj) < 2 {
		return Problem{}, fmt.Errorf("empty object")
	}
	obj = obj[1 : len(obj)-1]

	m := map[string]string{}
	for _, match := range jsKeyValueRe.FindAllStringSubmatch(obj, -1) {
		m[match[1]] = match[2]
	}
	bools := map[string]bool{}
	for _, match := range jsBoolRe.FindAllStringSubmatch(obj, -1) {
		bools[match[1]] = match[2] == "!0" || match[2] == "true"
	}

	title := m["problem"]
	if title == "" {
		return Problem{}, fmt.Errorf("missing problem field")
	}

	link := m["link"]
	video := m["video"]
	ncLink := m["ncLink"]
	if ncLink == "" {
		ncLink = link
	}

	leetURL := ""
	if link != "" {
		leetURL = leetcodeBase + link
	}
	videoURL := ""
	if video != "" {
		videoURL = "https://youtu.be/" + video
	}
	ncURL := ""
	if ncLink != "" {
		ncURL = neetcodeBase + ncLink
	}

	return Problem{
		Title:         title,
		Difficulty:    m["difficulty"],
		Category:      m["pattern"],
		VideoURL:      videoURL,
		LeetcodeURL:   leetURL,
		NeetcodeURL:   ncURL,
		IsNeetcode150: bools["neetcode150"],
		IsBlind75:     bools["blind75"],
		IsNeetcode250: bools["neetcode250"],
	}, nil
}

// ─── HTTP transport ───────────────────────────────────────────────────────────

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
