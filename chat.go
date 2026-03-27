package notebooklm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ChatAPI provides conversation operations for NotebookLM notebooks.
type ChatAPI struct {
	client *Client
}

// AskOptions configures a chat question.
type AskOptions struct {
	// SourceIDs filters the question to specific sources. Empty means all sources.
	SourceIDs []string
	// ConversationID continues an existing conversation (for follow-ups).
	ConversationID string
}

// Ask submits a question to a notebook and returns the answer with citations.
func (c *ChatAPI) Ask(ctx context.Context, notebookID, question string, opts AskOptions) (*AskResult, error) {
	// Use the GenerateFreeFormStreamed endpoint (QUERY_URL)
	c.client.mu.Lock()
	cookieHeader := c.client.auth.CookieHeader()
	csrfToken := c.client.auth.CSRFToken
	c.client.mu.Unlock()

	// Build request body for the query endpoint
	queryBody := buildQueryBody(notebookID, question, opts)
	bodyBytes, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rpc.QueryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create query request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("X-Goog-Csrf-Token", csrfToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := c.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read query response: %w", err)
	}

	return parseQueryResponse(body)
}

// Configure sets chat behavior for a notebook.
func (c *ChatAPI) Configure(ctx context.Context, notebookID string, goal rpc.ChatGoal, responseLength rpc.ChatResponseLength, customPrompt string) error {
	var goalParams any = int(goal)
	if goal == rpc.ChatCustom && customPrompt != "" {
		goalParams = []any{int(goal), customPrompt}
	}

	params := []any{
		notebookID,
		[]any{[]any{nil, nil, nil, []any{goalParams, int(responseLength)}}},
	}
	_, err := c.client.RPCCall(ctx, rpc.RenameNotebook, params, "/notebook/"+notebookID, true)
	return err
}

// SetMode applies a predefined chat mode.
func (c *ChatAPI) SetMode(ctx context.Context, notebookID string, mode ChatMode) error {
	var goal rpc.ChatGoal
	var length rpc.ChatResponseLength

	switch mode {
	case ChatModeDefault:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseDefault
	case ChatModeLearningGuide:
		goal = rpc.ChatLearningGuide
		length = rpc.ChatResponseDefault
	case ChatModeConcise:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseShorter
	case ChatModeDetailed:
		goal = rpc.ChatDefault
		length = rpc.ChatResponseLonger
	default:
		return fmt.Errorf("unknown chat mode: %s", mode)
	}

	return c.Configure(ctx, notebookID, goal, length, "")
}

// GetConversationTurns retrieves Q&A turns for a conversation.
func (c *ChatAPI) GetConversationTurns(ctx context.Context, notebookID, conversationID string, limit int) ([]ConversationTurn, error) {
	params := []any{notebookID, conversationID, limit}
	result, err := c.client.RPCCall(ctx, rpc.GetConversationTurns, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("get conversation turns: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	return parseConversationTurns(result), nil
}

// GetLastConversationID returns the most recent conversation ID for a notebook.
func (c *ChatAPI) GetLastConversationID(ctx context.Context, notebookID string) (string, error) {
	params := []any{notebookID}
	result, err := c.client.RPCCall(ctx, rpc.GetLastConversationID, params, "/notebook/"+notebookID, true)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	arr, _ := result.([]any)
	if len(arr) > 0 {
		if id, ok := arr[0].(string); ok {
			return id, nil
		}
	}
	return "", nil
}

func buildQueryBody(notebookID, question string, opts AskOptions) map[string]any {
	body := map[string]any{
		"notebookId": notebookID,
		"query":      question,
	}
	if len(opts.SourceIDs) > 0 {
		body["sourceIds"] = opts.SourceIDs
	}
	if opts.ConversationID != "" {
		body["conversationId"] = opts.ConversationID
	}
	return body
}

func parseQueryResponse(data []byte) (*AskResult, error) {
	// The response is a streaming proto-JSON format; we attempt to parse what we can.
	// NotebookLM's query endpoint returns newline-delimited JSON chunks.
	result := &AskResult{}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}

		// Extract answer text
		if text, ok := extractNestedString(chunk, "text"); ok && result.Answer == "" {
			result.Answer = text
		}
		// Extract conversation ID
		if convID, ok := extractNestedString(chunk, "conversationId"); ok {
			result.ConversationID = convID
		}
	}

	if result.Answer == "" && len(data) > 0 {
		// Fallback: return raw response as answer
		result.Answer = string(data)
	}

	return result, nil
}

func extractNestedString(m map[string]any, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	for _, v := range m {
		if nested, ok := v.(map[string]any); ok {
			if s, found := extractNestedString(nested, key); found {
				return s, true
			}
		}
	}
	return "", false
}

func parseConversationTurns(result any) []ConversationTurn {
	arr, _ := result.([]any)
	turns := make([]ConversationTurn, 0)
	for i, item := range arr {
		if turn, ok := item.([]any); ok {
			t := ConversationTurn{TurnNumber: i + 1}
			if len(turn) > 0 {
				if q, ok := turn[0].(string); ok {
					t.Query = q
				}
			}
			if len(turn) > 1 {
				if a, ok := turn[1].(string); ok {
					t.Answer = a
				}
			}
			turns = append(turns, t)
		}
	}
	return turns
}
