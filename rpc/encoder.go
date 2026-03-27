package rpc

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// EncodeRPCRequest encodes an RPC request into batchexecute triple-nested array format:
// [[[rpc_id, json_params, null, "generic"]]]
func EncodeRPCRequest(method RPCMethod, params any) ([]any, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	inner := []any{string(method), string(paramsJSON), nil, "generic"}
	return []any{[]any{inner}}, nil
}

// BuildRequestBody creates form-encoded request body for batchexecute.
// Format: f.req=<url-encoded-json>[&at=<csrf>]&
func BuildRequestBody(rpcRequest []any, csrfToken string) (string, error) {
	reqJSON, err := json.Marshal(rpcRequest)
	if err != nil {
		return "", fmt.Errorf("marshal rpc request: %w", err)
	}
	body := "f.req=" + url.QueryEscape(string(reqJSON))
	if csrfToken != "" {
		body += "&at=" + url.QueryEscape(csrfToken)
	}
	body += "&"
	return body, nil
}

// BuildURLParams constructs query parameters for batchexecute request.
func BuildURLParams(method RPCMethod, sourcePath, sessionID string) url.Values {
	if sourcePath == "" {
		sourcePath = "/"
	}
	params := url.Values{
		"rpcids":      {string(method)},
		"source-path": {sourcePath},
		"hl":          {"en"},
		"rt":          {"c"},
	}
	if sessionID != "" {
		params.Set("f.sid", sessionID)
	}
	return params
}
