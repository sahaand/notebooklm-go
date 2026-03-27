package notebooklm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

var youtubePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/shorts/|youtube\.com/embed/|youtube\.com/live/)([a-zA-Z0-9_-]{11})`),
}

// SourcesAPI provides source management operations.
type SourcesAPI struct {
	client *Client
}

// List returns all sources in a notebook.
func (s *SourcesAPI) List(ctx context.Context, notebookID string) ([]Source, error) {
	params := []any{notebookID, nil, []any{2}, nil, 0}
	result, err := s.client.RPCCall(ctx, rpc.GetNotebook, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("list sources for notebook %s: %w", notebookID, err)
	}

	arr, _ := result.([]any)
	if len(arr) < 2 {
		return nil, nil
	}

	// Sources are typically at result[1] in the GET_NOTEBOOK response
	sourcesRaw, _ := arr[1].([]any)
	sources := make([]Source, 0, len(sourcesRaw))
	for _, item := range sourcesRaw {
		if data, ok := item.([]any); ok {
			src := parseSource(data, notebookID)
			sources = append(sources, src)
		}
	}
	return sources, nil
}

// Get fetches a single source by ID.
func (s *SourcesAPI) Get(ctx context.Context, notebookID, sourceID string) (*Source, error) {
	params := []any{notebookID, sourceID}
	result, err := s.client.RPCCall(ctx, rpc.GetSource, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("get source %s: %w", sourceID, err)
	}
	data, _ := result.([]any)
	src := parseSource(data, notebookID)
	return &src, nil
}

// AddURL adds a web URL or YouTube video as a source.
func (s *SourcesAPI) AddURL(ctx context.Context, notebookID, rawURL string) (*Source, error) {
	isYT := isYouTubeURL(rawURL)
	var params []any
	if isYT {
		params = []any{notebookID, []any{[]any{nil, nil, rawURL}}, []any{2}}
	} else {
		params = []any{notebookID, []any{[]any{rawURL}}, []any{2}}
	}

	result, err := s.client.RPCCall(ctx, rpc.AddSource, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("add URL source: %w", err)
	}
	data, _ := result.([]any)
	src := parseSource(data, notebookID)
	return &src, nil
}

// AddText adds pasted text content as a source.
func (s *SourcesAPI) AddText(ctx context.Context, notebookID, title, content string) (*Source, error) {
	params := []any{notebookID, []any{[]any{nil, title, nil, content}}, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.AddSource, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("add text source: %w", err)
	}
	data, _ := result.([]any)
	src := parseSource(data, notebookID)
	return &src, nil
}

// AddFile uploads a file and adds it as a source.
// Supported formats: PDF, TXT, MD, DOCX.
func (s *SourcesAPI) AddFile(ctx context.Context, notebookID, filePath string) (*Source, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	uploadID, err := s.uploadFile(ctx, fileData, filePath)
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	// Register uploaded file as source
	params := []any{notebookID, []any{[]any{uploadID}}, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.AddSourceFile, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("register file source: %w", err)
	}
	data, _ := result.([]any)
	src := parseSource(data, notebookID)
	return &src, nil
}

// AddDrive adds a Google Drive document as a source.
func (s *SourcesAPI) AddDrive(ctx context.Context, notebookID, driveURL, mimeType string) (*Source, error) {
	params := []any{notebookID, []any{[]any{nil, nil, nil, nil, driveURL, mimeType}}, []any{2}}
	result, err := s.client.RPCCall(ctx, rpc.AddSource, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("add drive source: %w", err)
	}
	data, _ := result.([]any)
	src := parseSource(data, notebookID)
	return &src, nil
}

// Delete removes a source from a notebook.
func (s *SourcesAPI) Delete(ctx context.Context, notebookID, sourceID string) error {
	params := []any{notebookID, []any{sourceID}, []any{2}}
	_, err := s.client.RPCCall(ctx, rpc.DeleteSource, params, "/notebook/"+notebookID, true)
	if err != nil {
		return fmt.Errorf("delete source %s: %w", sourceID, err)
	}
	return nil
}

// Rename updates the title of a source.
func (s *SourcesAPI) Rename(ctx context.Context, notebookID, sourceID, newTitle string) (*Source, error) {
	params := []any{notebookID, sourceID, newTitle}
	_, err := s.client.RPCCall(ctx, rpc.UpdateSource, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("rename source %s: %w", sourceID, err)
	}
	return s.Get(ctx, notebookID, sourceID)
}

// Refresh re-fetches content for a URL or Drive source.
func (s *SourcesAPI) Refresh(ctx context.Context, notebookID, sourceID string) error {
	params := []any{notebookID, sourceID}
	_, err := s.client.RPCCall(ctx, rpc.RefreshSource, params, "/notebook/"+notebookID, true)
	return err
}

// WaitUntilReady polls a source until it's ready or an error occurs.
// Uses exponential backoff with a maximum interval.
func (s *SourcesAPI) WaitUntilReady(ctx context.Context, notebookID, sourceID string, timeout time.Duration) (*Source, error) {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	maxInterval := 15 * time.Second

	for {
		src, err := s.Get(ctx, notebookID, sourceID)
		if err != nil {
			return nil, err
		}
		if src.IsReady() {
			return src, nil
		}
		if src.IsError() {
			return nil, fmt.Errorf("source %s processing failed", sourceID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for source %s to be ready", sourceID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		interval = time.Duration(math.Min(float64(interval*2), float64(maxInterval)))
	}
}

func (s *SourcesAPI) uploadFile(ctx context.Context, data []byte, name string) (string, error) {
	s.client.mu.Lock()
	cookieHeader := s.client.auth.CookieHeader()
	s.client.mu.Unlock()

	// Initiate resumable upload
	uploadURL := rpc.UploadURL + "LabsTailwindUi/upload/homescreen"
	initReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, nil)
	if err != nil {
		return "", err
	}
	initReq.Header.Set("Cookie", cookieHeader)
	initReq.Header.Set("X-Goog-Upload-Protocol", "resumable")
	initReq.Header.Set("X-Goog-Upload-Command", "start")
	initReq.Header.Set("X-Goog-Upload-Content-Length", fmt.Sprintf("%d", len(data)))
	initReq.Header.Set("Content-Type", "application/octet-stream")
	initReq.Header.Set("X-Goog-Upload-Header-Content-Type", mimeTypeForFile(name))

	resp, err := s.client.httpClient.Do(initReq)
	if err != nil {
		return "", fmt.Errorf("initiate upload: %w", err)
	}
	resp.Body.Close()

	uploadSessionURL := resp.Header.Get("X-Goog-Upload-URL")
	if uploadSessionURL == "" {
		return "", fmt.Errorf("no upload session URL returned")
	}

	// Upload file data
	uploadReq, err := http.NewRequestWithContext(ctx, "POST", uploadSessionURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	uploadReq.Header.Set("Cookie", cookieHeader)
	uploadReq.Header.Set("X-Goog-Upload-Command", "upload, finalize")
	uploadReq.Header.Set("X-Goog-Upload-Offset", "0")
	uploadReq.Header.Set("Content-Type", "application/octet-stream")

	uploadResp, err := s.client.httpClient.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("upload data: %w", err)
	}
	defer uploadResp.Body.Close()

	body, _ := io.ReadAll(uploadResp.Body)
	// The upload ID is typically in the response body or a header
	uploadID := strings.TrimSpace(string(body))
	return uploadID, nil
}

func isYouTubeURL(rawURL string) bool {
	for _, pattern := range youtubePatterns {
		if pattern.MatchString(rawURL) {
			return true
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.Contains(host, "youtube.com") || host == "youtu.be"
}

func mimeTypeForFile(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".docx"):
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case strings.HasSuffix(lower, ".md"):
		return "text/markdown"
	default:
		return "text/plain"
	}
}

func parseSource(data []any, notebookID string) Source {
	src := Source{NotebookID: notebookID}
	if len(data) == 0 {
		return src
	}
	if id, ok := data[0].(string); ok {
		src.ID = id
	}
	if len(data) > 1 {
		if info, ok := data[1].([]any); ok {
			if len(info) > 0 {
				if title, ok := info[0].(string); ok {
					src.Title = title
				}
			}
			if len(info) > 1 {
				if u, ok := info[1].(string); ok {
					src.URL = u
				}
			}
		}
	}
	// Status at data[3][1]
	if len(data) > 3 {
		if statusArr, ok := data[3].([]any); ok && len(statusArr) > 1 {
			if code, ok := statusArr[1].(float64); ok {
				src.Status = rpc.SourceStatus(int(code))
			}
		}
	}
	return src
}
