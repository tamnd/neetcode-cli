package neetcode_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/neetcode-cli/neetcode"
)

// fakeBundle simulates the relevant portion of the NeetCode Angular bundle.
const fakeBundle = `
var ye="https://leetcode.com/problems/",F="https://youtube.com/embed/",Q=[{problem:"Contains Duplicate",pattern:"Arrays & Hashing",link:"contains-duplicate/",video:"3OamzN90kPg",difficulty:"Easy",code:"0217-contains-duplicate",neetcode150:!0,blind75:!0,neetcode250:!0,ncLink:"duplicate-integer/"},{problem:"Valid Anagram",pattern:"Arrays & Hashing",link:"valid-anagram/",video:"9UtInBqnCgA",difficulty:"Easy",code:"0242-valid-anagram",neetcode150:!0,blind75:!0,neetcode250:!0,ncLink:"is-anagram/"},{problem:"Two Sum",pattern:"Arrays & Hashing",link:"two-sum/",video:"KLlXCFG5TnA",difficulty:"Easy",code:"0001-two-sum",neetcode150:!0,blind75:!0,neetcode250:!0,ncLink:"two-sum/"}]
`

// fakeIndexHTML returns an index page pointing to our fake bundle.
func fakeIndexHTML(bundlePath string) string {
	return `<html><head></head><body><script src="` + bundlePath + `"></script></body></html>`
}

// newTestServer returns an httptest.Server that serves a fake index and bundle.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	const bundleName = "main.abc123def456.js"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/"+bundleName {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(fakeBundle))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fakeIndexHTML(bundleName)))
	})
	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, baseURL string) *neetcode.Client {
	t.Helper()
	cfg := neetcode.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	return neetcode.NewClient(cfg)
}

func TestProblemsReturnsAll(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	probs, err := c.Problems(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) != 3 {
		t.Fatalf("got %d problems, want 3", len(probs))
	}
}

func TestProblemsLimit(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	probs, err := c.Problems(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(probs) != 2 {
		t.Fatalf("got %d problems, want 2", len(probs))
	}
}

func TestProblemFields(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	probs, err := c.Problems(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	p := probs[0]
	if p.Title != "Contains Duplicate" {
		t.Errorf("title = %q", p.Title)
	}
	if p.Difficulty != "Easy" {
		t.Errorf("difficulty = %q", p.Difficulty)
	}
	if p.Category != "Arrays & Hashing" {
		t.Errorf("category = %q", p.Category)
	}
	if !strings.Contains(p.LeetcodeURL, "contains-duplicate") {
		t.Errorf("leetcode_url = %q", p.LeetcodeURL)
	}
	if !strings.Contains(p.VideoURL, "3OamzN90kPg") {
		t.Errorf("video_url = %q", p.VideoURL)
	}
	if !p.IsNeetcode150 {
		t.Error("expected neetcode_150 = true")
	}
	if !p.IsBlind75 {
		t.Error("expected blind_75 = true")
	}
}

func TestSearchByTitle(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	results, err := c.Search(context.Background(), "anagram", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results for 'anagram', want 1", len(results))
	}
	if results[0].Title != "Valid Anagram" {
		t.Errorf("got %q", results[0].Title)
	}
}

func TestSearchByCategory(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	results, err := c.Search(context.Background(), "arrays", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results for 'arrays', want 3", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	results, err := c.Search(context.Background(), "arrays", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
}

func TestGetSendsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fakeIndexHTML("main.abc123def456.js")))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	// Ignore errors; we only care that the UA was sent on the first request.
	_, _ = c.Problems(context.Background(), 1)
	if gotUA == "" {
		t.Error("request carried no User-Agent")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := neetcode.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := neetcode.NewClient(cfg)

	start := time.Now()
	_, _ = c.Problems(context.Background(), 1) // will fail but exercises retries
	if hits < 3 {
		t.Errorf("server saw %d hits, want >= 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}
