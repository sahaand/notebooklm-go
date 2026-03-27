package rpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// StripAntiXSSI removes the )]}' prefix Google adds to prevent XSSI attacks.
func StripAntiXSSI(response string) string {
	if strings.HasPrefix(response, ")]}'\n") {
		return response[5:]
	}
	if strings.HasPrefix(response, ")]}'\r\n") {
		return response[6:]
	}
	return response
}

// ParseChunkedResponse parses the alternating byte-count/JSON chunked format (rt=c mode).
func ParseChunkedResponse(response string) ([]any, error) {
	if strings.TrimSpace(response) == "" {
		return nil, nil
	}

	var chunks []any
	skipped := 0
	lines := strings.Split(strings.TrimSpace(response), "\n")

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		// Try to parse as byte count
		if _, err := strconv.Atoi(line); err == nil {
			i++
			if i < len(lines) {
				var chunk any
				if err := json.Unmarshal([]byte(lines[i]), &chunk); err != nil {
					skipped++
				} else {
					chunks = append(chunks, chunk)
				}
			}
			i++
			continue
		}

		// Not a byte count — try parsing as JSON directly
		var chunk any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			skipped++
		} else {
			chunks = append(chunks, chunk)
		}
		i++
	}

	if skipped > 0 && len(lines) > 0 {
		rate := float64(skipped) / float64(len(lines))
		if rate > 0.1 {
			return nil, fmt.Errorf("response parsing failed: %d of %d chunks malformed (%.0f%%)", skipped, len(lines), rate*100)
		}
	}

	return chunks, nil
}

// ExtractRPCResult finds and returns result data for a specific RPC method ID from chunks.
func ExtractRPCResult(chunks []any, rpcID string) (any, error) {
	for _, chunk := range chunks {
		items, ok := toSlice(chunk)
		if !ok {
			continue
		}

		// The outer array may contain items directly or wrap them in another array
		var candidates [][]any
		if len(items) > 0 {
			if _, ok := items[0].([]any); ok {
				// Outer is a list of items
				for _, item := range items {
					if s, ok := toSlice(item); ok {
						candidates = append(candidates, s)
					}
				}
			} else {
				// items itself is an item
				candidates = [][]any{items}
			}
		}

		for _, item := range candidates {
			if len(item) < 3 {
				continue
			}
			tag, _ := item[0].(string)
			id, _ := item[1].(string)

			if tag == "er" && id == rpcID {
				code := item[2]
				return nil, &RPCError{
					Message:  errorMessageForCode(code),
					MethodID: rpcID,
					RPCCode:  code,
				}
			}

			if tag == "wrb.fr" && id == rpcID {
				resultData := item[2]

				// Check for UserDisplayableError (rate limit) when result is nil
				if resultData == nil && len(item) > 5 && item[5] != nil {
					if containsUserDisplayableError(item[5]) {
						return nil, &RPCError{
							Message:  "API rate limit or quota exceeded. Please wait before retrying.",
							MethodID: rpcID,
							RPCCode:  "USER_DISPLAYABLE_ERROR",
						}
					}
				}

				// If resultData is a string, try JSON-parsing it
				if s, ok := resultData.(string); ok {
					var parsed any
					if err := json.Unmarshal([]byte(s), &parsed); err == nil {
						return parsed, nil
					}
					return s, nil
				}
				return resultData, nil
			}
		}
	}
	return nil, nil
}

// DecodeResponse is the full pipeline: strip prefix -> parse chunks -> extract result.
func DecodeResponse(raw, rpcID string) (any, error) {
	cleaned := StripAntiXSSI(raw)
	chunks, err := ParseChunkedResponse(cleaned)
	if err != nil {
		return nil, fmt.Errorf("parse chunked response: %w", err)
	}
	result, err := ExtractRPCResult(chunks, rpcID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func toSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func containsUserDisplayableError(v any) bool {
	switch val := v.(type) {
	case string:
		return strings.Contains(val, "UserDisplayableError")
	case []any:
		for _, item := range val {
			if containsUserDisplayableError(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range val {
			if containsUserDisplayableError(item) {
				return true
			}
		}
	}
	return false
}

func errorMessageForCode(code any) string {
	if code == nil {
		return "unknown error"
	}
	if n, ok := code.(float64); ok {
		switch int(n) {
		case 400:
			return "invalid request parameters"
		case 401:
			return "authentication required — run 'notebooklm login'"
		case 403:
			return "insufficient permissions"
		case 404:
			return "resource not found"
		case 429:
			return "API rate limit exceeded — please wait before retrying"
		case 500:
			return "server error — usually temporary, try again later"
		}
	}
	return fmt.Sprintf("error code: %v", code)
}

// RPCError represents an error returned by the RPC API.
type RPCError struct {
	Message  string
	MethodID string
	RPCCode  any
}

func (e *RPCError) Error() string {
	if e.MethodID != "" {
		return fmt.Sprintf("RPC %s error: %s", e.MethodID, e.Message)
	}
	return e.Message
}
