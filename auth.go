package notebooklm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	csrfPattern      = regexp.MustCompile(`"SNlM0e"\s*:\s*"([^"]+)"`)
	sessionIDPattern = regexp.MustCompile(`"FdrFJe"\s*:\s*"([^"]+)"`)
)

// AuthTokens holds authentication credentials for the NotebookLM API.
type AuthTokens struct {
	// Cookies maps cookie names to values loaded from Playwright storage state.
	Cookies map[string]string
	// CSRFToken is the SNlM0e token required for all RPC calls.
	CSRFToken string
	// SessionID is the FdrFJe token passed in URL query parameters.
	SessionID string
}

// CookieHeader returns a semicolon-separated Cookie header value.
func (a *AuthTokens) CookieHeader() string {
	parts := make([]string, 0, len(a.Cookies))
	for k, v := range a.Cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// DefaultStoragePath returns the default path for the Playwright storage state file.
func DefaultStoragePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	if h := os.Getenv("NOTEBOOKLM_HOME"); h != "" {
		home = h
	}
	return filepath.Join(home, ".notebooklm", "storage_state.json")
}

// LoadAuthFromStorage loads Google cookies from Playwright storage state.
// Priority:
//  1. Explicit path argument
//  2. NOTEBOOKLM_AUTH_JSON environment variable (inline JSON)
//  3. File at $NOTEBOOKLM_HOME/storage_state.json or ~/.notebooklm/storage_state.json
func LoadAuthFromStorage(path string) (map[string]string, error) {
	var data []byte
	var err error

	switch {
	case path != "":
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read storage file %s: %w", path, err)
		}
	case os.Getenv("NOTEBOOKLM_AUTH_JSON") != "":
		data = []byte(os.Getenv("NOTEBOOKLM_AUTH_JSON"))
	default:
		defaultPath := DefaultStoragePath()
		data, err = os.ReadFile(defaultPath)
		if err != nil {
			return nil, fmt.Errorf("storage file not found: %s — run 'notebooklm login' to authenticate", defaultPath)
		}
	}

	var storageState struct {
		Cookies []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal(data, &storageState); err != nil {
		return nil, fmt.Errorf("parse storage state: %w", err)
	}

	return extractCookies(storageState.Cookies), nil
}

// allowedDomains returns the set of domains from which we accept cookies.
func isAllowedDomain(domain string) bool {
	allowed := []string{
		".google.com",
		"notebooklm.google.com",
		".googleusercontent.com",
	}
	for _, a := range allowed {
		if domain == a {
			return true
		}
	}
	// Regional Google ccTLDs
	if strings.HasPrefix(domain, ".google.") {
		return true
	}
	return false
}

func extractCookies(raw []struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}) map[string]string {
	cookies := make(map[string]string)
	domains := make(map[string]string) // cookie name -> domain it came from

	for _, c := range raw {
		if !isAllowedDomain(c.Domain) || c.Name == "" {
			continue
		}
		isBase := c.Domain == ".google.com"
		if _, exists := cookies[c.Name]; !exists || isBase {
			cookies[c.Name] = c.Value
			domains[c.Name] = c.Domain
		}
	}
	_ = domains
	return cookies
}

// FetchTokens fetches CSRF and session ID tokens from the NotebookLM homepage.
func FetchTokens(ctx context.Context, cookies map[string]string) (csrfToken, sessionID string, err error) {
	cookieHeader := makeCookieHeader(cookies)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://notebooklm.google.com/", nil)
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch notebooklm home: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	html := string(body)
	if strings.Contains(resp.Request.URL.String(), "accounts.google.com") ||
		strings.Contains(html, "accounts.google.com/signin") {
		return "", "", fmt.Errorf("authentication expired — run 'notebooklm login' to re-authenticate")
	}

	m := csrfPattern.FindStringSubmatch(html)
	if m == nil {
		return "", "", fmt.Errorf("CSRF token (SNlM0e) not found in page HTML")
	}
	csrfToken = m[1]

	m = sessionIDPattern.FindStringSubmatch(html)
	if m == nil {
		return "", "", fmt.Errorf("session ID (FdrFJe) not found in page HTML")
	}
	sessionID = m[1]

	return csrfToken, sessionID, nil
}

// NewAuthTokensFromStorage creates AuthTokens by loading storage and fetching live tokens.
// If the storage file contains pre-extracted tokens (written by scripts/auth/export_cookies.js) those are
// used directly, avoiding raw HTTP requests that Google blocks with CookieMismatch.
func NewAuthTokensFromStorage(ctx context.Context, storagePath string) (*AuthTokens, error) {
	cookies, csrf, sid, err := loadAuthAndTokens(storagePath)
	if err != nil {
		return nil, err
	}
	if csrf == "" || sid == "" {
		csrf, sid, err = FetchTokens(ctx, cookies)
		if err != nil {
			return nil, err
		}
	}
	return &AuthTokens{
		Cookies:   cookies,
		CSRFToken: csrf,
		SessionID: sid,
	}, nil
}

func loadAuthAndTokens(path string) (cookies map[string]string, csrfToken, sessionID string, err error) {
	var data []byte

	switch {
	case path != "":
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, "", "", fmt.Errorf("read storage file %s: %w", path, err)
		}
	case os.Getenv("NOTEBOOKLM_AUTH_JSON") != "":
		data = []byte(os.Getenv("NOTEBOOKLM_AUTH_JSON"))
	default:
		defaultPath := DefaultStoragePath()
		data, err = os.ReadFile(defaultPath)
		if err != nil {
			return nil, "", "", fmt.Errorf("storage file not found: %s — run 'notebooklm login' to authenticate", defaultPath)
		}
	}

	var storageState struct {
		Cookies []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
		} `json:"cookies"`
		NotebookLM *struct {
			CSRFToken string `json:"csrfToken"`
			SessionID string `json:"sessionID"`
		} `json:"_notebooklm"`
	}
	if err := json.Unmarshal(data, &storageState); err != nil {
		return nil, "", "", fmt.Errorf("parse storage state: %w", err)
	}

	cookies = extractCookies(storageState.Cookies)
	if storageState.NotebookLM != nil {
		csrfToken = storageState.NotebookLM.CSRFToken
		sessionID = storageState.NotebookLM.SessionID
	}
	return cookies, csrfToken, sessionID, nil
}

func makeCookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}
