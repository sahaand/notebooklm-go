# WIP — Remaining Implementation Tasks

This file tracks known gaps between this Go implementation and the Python
reference (`teng-lin/notebooklm-py`), along with notes on how to implement
each one.

Current overall completeness vs Python library: **~76/100**
For the Claude + Obsidian use case specifically: **~90/100**

---

## Critical / High Priority

### 1. Citation parsing in `chat.Ask()` responses
**File:** `chat.go` — `parseQueryResponse()` (lines ~152–183)

**Problem:** The streaming response from `rpc.QueryURL` contains citation
references, but the current parser only extracts `"text"` and `"conversationId"`.
`AskResult.References []ChatReference` is always empty.

**How to fix:**
- Capture a live network response from notebooklm.google.com with browser
  DevTools while asking a question
- Identify the JSON keys for citation arrays (likely something like
  `"citations"`, `"references"`, or nested inside each chunk)
- Parse `source_id`, `cited_text`, `start_char`, `end_char` into
  `[]ChatReference` and populate `AskResult.References`

**Types already defined:** `ChatReference` in `types.go` is ready.

---

### 2. Mind map JSON structure extraction
**File:** `notes.go` — `ListMindMaps()` (returns `[]any`)

**Problem:** `ListMindMaps()` returns raw `[]any` without parsing the
hierarchical node structure. There is no way to programmatically walk or
export a mind map as JSON.

**How to fix:**
- Add a `MindMapNode` type to `types.go`:
  ```go
  type MindMapNode struct {
      ID       string        `json:"id"`
      Label    string        `json:"label"`
      Children []MindMapNode `json:"children,omitempty"`
  }
  ```
- Write a `parseMindMap(data []any) *MindMapNode` parser by inspecting the
  raw response structure (use `--json` output on a real notebook with a
  generated mind map to see the shape)
- Add a CLI command: `notebooklm notes mind-map export <artifact-id> -n <id>`
  that writes the JSON tree to stdout or a file

---

### 3. `source get-fulltext` — indexed text retrieval
**File:** `sources.go` (method missing); `rpc/types.go` (RPC ID unknown)

**Problem:** The Python library has `sources.get_fulltext()` which returns the
full indexed text content NotebookLM uses internally. The Go RPC method ID has
not been reverse-engineered.

**How to find the RPC ID:**
1. Open notebooklm.google.com in Chrome
2. DevTools → Network tab → filter by `batchexecute`
3. Click on a source in the web UI to trigger a fulltext load
4. Find the request with a single-source param — note the `rpcids` field
5. Add the method ID to `rpc/types.go` as `GetSourceFulltext`

**Implementation once RPC ID is known:**
```go
// In sources.go
func (s *SourcesAPI) GetFulltext(ctx context.Context, notebookID, sourceID string) (string, error) {
    params := []any{notebookID, sourceID}
    result, err := s.client.RPCCall(ctx, rpc.GetSourceFulltext, params, "/notebook/"+notebookID, false)
    // parse result.([]any)[0].(string)
}
```

**CLI command to add to `source_cmd.go`:**
```
notebooklm source get-fulltext <source-id> -n <notebook-id>
```

---

## Medium Priority

### 4. Structured artifact exports (quiz, flashcard, data table)
**File:** `artifacts.go`

**Problem:** The Python library exports quizzes as JSON/Markdown/HTML and
flashcards in multiple formats. Currently `Download()` just writes raw bytes.

**How to fix:**
- Determine the export format from `Artifact.TypeCode`
- For quiz/flashcard: parse the downloaded content (likely JSON) and provide
  format conversion helpers
- For data table: the raw download is likely CSV — no change needed
- Add `--format json|markdown|html` flag to `artifact download` command

### 5. Report markdown-native export
**Problem:** Generated reports download as binary. For Obsidian integration
it would be ideal to get clean markdown.

**Note:** The underlying content may already be markdown — investigate by
downloading a report and checking the raw bytes. If it is markdown, just
change the file extension handling in `Download()`.

---

## Low Priority

### 6. Batch source operations
Not in the current RPC surface. Would require multiple sequential API calls
wrapped in a helper.

### 7. Chat personas / custom system prompt
**File:** `chat.go` — `Configure()` uses `rpc.RenameNotebook` (likely wrong
RPC method). Needs investigation to find the correct method for setting a
custom chat persona/system prompt.

### 8. `source discover` response parsing
**File:** `sources.go` — `DiscoverSources()` returns `[]string` but the actual
RPC response shape is unknown. May need adjustment once tested against a live
notebook.

### 9. Slide revision (`artifact revise-slide`) parameter structure
**File:** `artifacts.go` — `ReviseSlide()` parameters are best-effort. The
exact nesting (`slideIndex` may be 0-based or 1-based, `instructions` may need
to be wrapped in `[]any`) needs verification against live traffic.

---

## Scores Summary

| Area | Score | Notes |
|---|---|---|
| Notebook management | 100% | Complete |
| Source management | 86% | Missing: fulltext |
| Chat & conversation | 67% | Missing: citations |
| Research | 100% | Complete |
| Sharing | 100% | Complete |
| Audio generation | 100% | Complete |
| Video generation | 100% | Complete |
| Report generation | 75% | Markdown export needs verification |
| Quiz/Flashcard | 50% | Generation works; no structured export |
| Infographic | 80% | Generation works; no PNG-specific handling |
| Slide deck | 70% | Generation works; revision untested |
| Data table | 100% | Complete |
| Mind maps | 60% | Generation works; JSON structure not parsed |
| Notes | 90% | Works; no bulk markdown export utility |
| Settings | 100% | Complete |
| CLI surface | 95% | ~57/60 commands |

**Overall: ~76/100**
