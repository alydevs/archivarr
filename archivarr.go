/*
   Copyright 2026 Aly Smith "alydevs" https://aly.pet

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ── Configuration ──────────────────────────────────────────────────────────────

const (
	listenAddr   = ":8080"
	upstream     = "https://archive.org/download"
	maxRedirects = 10
)

// authHeader is set at startup from IA_ACCESS and IA_SECRET env vars.
var authHeader string

// httpClient does not follow redirects automatically — we handle them manually
// so we can re-inject auth headers on each hop.
var httpClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// ── Helpers ────────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── Redirect-following fetch ───────────────────────────────────────────────────

// fetchWithRetry wraps fetchFollowingRedirects with exponential backoff on 401.
// It retries until the delay would exceed 60s, then returns the 401 response.
func fetchWithRetry(method, rawURL string, clientHeaders http.Header) (*http.Response, error) {
	delay := 2 * time.Second
	maxDelay := 60 * time.Second
	for {
		resp, err := fetchFollowingRedirects(method, rawURL, clientHeaders)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusUnauthorized || delay > maxDelay {
			return resp, nil
		}
		// Drain and close the 401 body before retrying.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		log.Printf("401 received, retrying in %s", delay)
		time.Sleep(delay)
		delay *= 2
	}
}

// fetchFollowingRedirects performs a request and manually follows any 3xx
// redirects, re-injecting the auth header on every hop.
func fetchFollowingRedirects(method, rawURL string, clientHeaders http.Header) (*http.Response, error) {
	for i := 0; i < maxRedirects; i++ {
		req, err := http.NewRequest(method, rawURL, nil)
		if err != nil {
			return nil, err
		}

		// Forward the original client headers (e.g. Range).
		for k, vals := range clientHeaders {
			for _, v := range vals {
				req.Header.Add(k, v)
			}
		}

		// Always inject auth header (overrides anything from the client).
		req.Header.Set("Authorization", authHeader)

		log.Printf("→ %s %s", method, rawURL)
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		switch resp.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound,
			http.StatusSeeOther, http.StatusTemporaryRedirect,
			http.StatusPermanentRedirect:

			location := resp.Header.Get("Location")
			if location == "" {
				return resp, nil // no Location header — return as-is
			}
			resp.Body.Close()

			// Resolve relative redirects against the current URL.
			base, _ := url.Parse(rawURL)
			loc, err := url.Parse(location)
			if err != nil {
				return nil, err
			}
			rawURL = base.ResolveReference(loc).String()
			//log.Printf("↪ redirect → %s", rawURL)

		default:
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				log.Printf("← %d %s\n    body: %s", resp.StatusCode, rawURL, truncate(string(body), 512))
				// Restore body so the caller can still forward it to the client.
				resp.Body = io.NopCloser(strings.NewReader(string(body)))
			} else {
				log.Printf("← %d %s", resp.StatusCode, rawURL)
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("too many redirects (max %d)", maxRedirects)
}

// ── Handler ────────────────────────────────────────────────────────────────────

func main() {
	access := os.Getenv("IA_ACCESS")
	secret := os.Getenv("IA_SECRET")
	if access == "" || secret == "" {
		log.Fatal("IA_ACCESS and IA_SECRET environment variables must be set")
	}
	authHeader = fmt.Sprintf("LOW %s:%s", access, secret)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		upstreamURL := upstream + r.URL.RequestURI()

		resp, err := fetchWithRetry(r.Method, upstreamURL, r.Header)
		if err != nil {
			log.Printf("fetch error for %s: %v", r.URL, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy upstream response headers to the client.
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	log.Printf("Listening on %s  →  %s", listenAddr, upstream)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
