package notebooklm

import (
	"context"
	"fmt"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ResearchAPI provides web research operations.
type ResearchAPI struct {
	client *Client
}

// ResearchResult represents the result of a research operation.
type ResearchResult struct {
	TaskID  string   `json:"task_id"`
	Status  string   `json:"status"`
	Sources []string `json:"sources,omitempty"`
}

// StartFast initiates a fast web research query.
func (r *ResearchAPI) StartFast(ctx context.Context, notebookID, query string) (*ResearchResult, error) {
	params := []any{notebookID, query}
	result, err := r.client.RPCCall(ctx, rpc.StartFastResearch, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("start fast research: %w", err)
	}
	return parseResearchResult(result), nil
}

// StartDeep initiates a deep web research query.
func (r *ResearchAPI) StartDeep(ctx context.Context, notebookID, query string) (*ResearchResult, error) {
	params := []any{notebookID, query}
	result, err := r.client.RPCCall(ctx, rpc.StartDeepResearch, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("start deep research: %w", err)
	}
	return parseResearchResult(result), nil
}

// Poll checks the status of a research task.
func (r *ResearchAPI) Poll(ctx context.Context, notebookID, taskID string) (*ResearchResult, error) {
	params := []any{notebookID, taskID}
	result, err := r.client.RPCCall(ctx, rpc.PollResearch, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("poll research: %w", err)
	}
	res := parseResearchResult(result)
	res.TaskID = taskID
	return res, nil
}

// Import imports research results as sources into a notebook.
func (r *ResearchAPI) Import(ctx context.Context, notebookID, taskID string, sourceURLs []string) error {
	params := []any{notebookID, taskID, sourceURLs}
	_, err := r.client.RPCCall(ctx, rpc.ImportResearch, params, "/notebook/"+notebookID, true)
	return err
}

// WaitForCompletion polls research until it's complete or times out.
func (r *ResearchAPI) WaitForCompletion(ctx context.Context, notebookID, taskID string, timeout time.Duration) (*ResearchResult, error) {
	deadline := time.Now().Add(timeout)
	interval := 3 * time.Second

	for {
		res, err := r.Poll(ctx, notebookID, taskID)
		if err != nil {
			return nil, err
		}
		if res.Status == "completed" {
			return res, nil
		}
		if res.Status == "failed" {
			return nil, fmt.Errorf("research task %s failed", taskID)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for research %s", taskID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func parseResearchResult(result any) *ResearchResult {
	res := &ResearchResult{Status: "pending"}
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return res
	}
	if id, ok := arr[0].(string); ok {
		res.TaskID = id
	}
	if len(arr) > 1 {
		if status, ok := arr[1].(string); ok {
			res.Status = status
		}
	}
	return res
}
