package notebooklm

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

// ArtifactsAPI provides artifact generation, retrieval, and download operations.
type ArtifactsAPI struct {
	client *Client
}

// List returns all artifacts in a notebook.
func (a *ArtifactsAPI) List(ctx context.Context, notebookID string) ([]*Artifact, error) {
	params := []any{[]any{2}, notebookID, `NOT artifact.status = "ARTIFACT_STATUS_SUGGESTED"`}
	result, err := a.client.RPCCall(ctx, rpc.ListArtifacts, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}

	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil, nil
	}
	raw, _ := arr[0].([]any)
	if raw == nil {
		raw = arr
	}

	artifacts := make([]*Artifact, 0, len(raw))
	for _, item := range raw {
		if data, ok := item.([]any); ok {
			if art := ArtifactFromAPIResponse(data); art != nil {
				artifacts = append(artifacts, art)
			}
		}
	}
	return artifacts, nil
}

// Get returns a single artifact by ID.
func (a *ArtifactsAPI) Get(ctx context.Context, notebookID, artifactID string) (*Artifact, error) {
	all, err := a.List(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	for _, art := range all {
		if art.ID == artifactID {
			return art, nil
		}
	}
	return nil, fmt.Errorf("artifact %s not found in notebook %s", artifactID, notebookID)
}

// Delete removes an artifact.
func (a *ArtifactsAPI) Delete(ctx context.Context, notebookID, artifactID string) error {
	params := []any{notebookID, artifactID}
	_, err := a.client.RPCCall(ctx, rpc.DeleteArtifact, params, "/notebook/"+notebookID, true)
	return err
}

// Rename updates the title of an artifact.
func (a *ArtifactsAPI) Rename(ctx context.Context, notebookID, artifactID, newTitle string) error {
	params := []any{notebookID, artifactID, newTitle}
	_, err := a.client.RPCCall(ctx, rpc.RenameArtifact, params, "/notebook/"+notebookID, true)
	return err
}

// GenerateAudio creates an audio overview.
func (a *ArtifactsAPI) GenerateAudio(ctx context.Context, notebookID string, opts GenerateAudioOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	var formatConfig []any
	if opts.Format != 0 || opts.Length != 0 {
		f := int(opts.Format)
		if f == 0 {
			f = int(rpc.AudioDeepDive)
		}
		l := int(opts.Length)
		if l == 0 {
			l = int(rpc.AudioDefault)
		}
		formatConfig = []any{f, l}
	}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeAudio, sourceParam, lang, opts.Instructions, formatConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateVideo creates a video overview.
func (a *ArtifactsAPI) GenerateVideo(ctx context.Context, notebookID string, opts GenerateVideoOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	var formatConfig []any
	if opts.Format != 0 || opts.Style != 0 {
		f := int(opts.Format)
		if f == 0 {
			f = int(rpc.VideoExplainer)
		}
		st := int(opts.Style)
		if st == 0 {
			st = int(rpc.VideoStyleAutoSelect)
		}
		formatConfig = []any{f, st}
	}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeVideo, sourceParam, lang, opts.Instructions, formatConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateReport creates a report artifact.
func (a *ArtifactsAPI) GenerateReport(ctx context.Context, notebookID string, opts GenerateReportOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	format := opts.Format
	if format == "" {
		format = rpc.ReportBriefingDoc
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	var customConfig []any
	if opts.CustomPrompt != "" || opts.ExtraInstructions != "" {
		customConfig = []any{opts.CustomPrompt, opts.ExtraInstructions}
	}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeReport, sourceParam, lang, string(format), customConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateQuiz creates a quiz artifact.
func (a *ArtifactsAPI) GenerateQuiz(ctx context.Context, notebookID string, opts GenerateQuizOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	q := int(opts.Quantity)
	if q == 0 {
		q = int(rpc.QuizStandard)
	}
	d := int(opts.Difficulty)
	if d == 0 {
		d = int(rpc.QuizMedium)
	}

	variant := 1 // quiz
	if opts.Flashcards {
		variant = 2
	}
	formatConfig := []any{q, d, variant}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeQuiz, sourceParam, lang, "", formatConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateInfographic creates an infographic artifact.
func (a *ArtifactsAPI) GenerateInfographic(ctx context.Context, notebookID string, opts GenerateInfographicOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	var formatConfig []any
	o := int(opts.Orientation)
	d := int(opts.DetailLevel)
	st := int(opts.Style)
	if o != 0 || d != 0 || st != 0 {
		if o == 0 {
			o = int(rpc.InfographicLandscape)
		}
		if d == 0 {
			d = int(rpc.InfographicStandard)
		}
		if st == 0 {
			st = int(rpc.InfographicStyleAutoSelect)
		}
		formatConfig = []any{o, d, st}
	}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeInfographic, sourceParam, lang, opts.Instructions, formatConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateSlideDeck creates a slide deck artifact.
func (a *ArtifactsAPI) GenerateSlideDeck(ctx context.Context, notebookID string, opts GenerateSlideDeckOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)

	var formatConfig []any
	f := int(opts.Format)
	l := int(opts.Length)
	if f != 0 || l != 0 {
		if f == 0 {
			f = int(rpc.SlideDeckDetailed)
		}
		if l == 0 {
			l = int(rpc.SlideDeckDefault)
		}
		formatConfig = []any{f, l}
	}

	params := buildArtifactParams(notebookID, rpc.ArtifactTypeSlideDeck, sourceParam, lang, "", formatConfig)
	return a.createArtifact(ctx, notebookID, params)
}

// GenerateDataTable creates a data table artifact.
func (a *ArtifactsAPI) GenerateDataTable(ctx context.Context, notebookID string, opts GenerateDataTableOptions) (*GenerationStatus, error) {
	lang := opts.Language
	if lang == "" {
		lang = "en"
	}
	sourceParam := buildSourceParam(opts.SourceIDs)
	params := buildArtifactParams(notebookID, rpc.ArtifactTypeDataTable, sourceParam, lang, opts.Instructions, nil)
	return a.createArtifact(ctx, notebookID, params)
}

// WaitForCompletion polls until an artifact generation task finishes.
func (a *ArtifactsAPI) WaitForCompletion(ctx context.Context, notebookID, taskID string, timeout time.Duration) (*GenerationStatus, error) {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	maxInterval := 10 * time.Second

	for {
		art, err := a.Get(ctx, notebookID, taskID)
		if err != nil {
			return nil, err
		}

		status := &GenerationStatus{
			TaskID: taskID,
			Status: rpc.ArtifactStatusToStr(art.Status),
			URL:    art.URL,
		}

		if art.IsCompleted() {
			return status, nil
		}
		if art.IsFailed() {
			return status, fmt.Errorf("artifact generation failed")
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for artifact %s completion", taskID)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		interval = time.Duration(math.Min(float64(interval*2), float64(maxInterval)))
	}
}

// Download downloads an artifact to a file.
func (a *ArtifactsAPI) Download(ctx context.Context, notebookID, artifactID, destPath string) error {
	art, err := a.Get(ctx, notebookID, artifactID)
	if err != nil {
		return err
	}
	if !art.IsCompleted() {
		return fmt.Errorf("artifact %s is not ready (status: %s)", artifactID, rpc.ArtifactStatusToStr(art.Status))
	}
	if art.URL == "" {
		return fmt.Errorf("artifact %s has no download URL", artifactID)
	}

	data, err := a.client.Download(ctx, art.URL)
	if err != nil {
		return fmt.Errorf("download artifact: %w", err)
	}

	return os.WriteFile(destPath, data, 0644)
}

// GetSuggestedReports returns AI-suggested report formats for a notebook.
func (a *ArtifactsAPI) GetSuggestedReports(ctx context.Context, notebookID string) ([]ReportSuggestion, error) {
	params := []any{notebookID}
	result, err := a.client.RPCCall(ctx, rpc.GetSuggestedReports, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("get suggested reports: %w", err)
	}
	arr, _ := result.([]any)
	suggestions := make([]ReportSuggestion, 0)
	for _, item := range arr {
		if data, ok := item.([]any); ok {
			s := ReportSuggestion{}
			if len(data) > 0 {
				if title, ok := data[0].(string); ok {
					s.Title = title
				}
			}
			if len(data) > 1 {
				if desc, ok := data[1].(string); ok {
					s.Description = desc
				}
			}
			if len(data) > 2 {
				if prompt, ok := data[2].(string); ok {
					s.Prompt = prompt
				}
			}
			suggestions = append(suggestions, s)
		}
	}
	return suggestions, nil
}

// ExportArtifact exports an artifact to Google Docs or Sheets and returns the URL.
func (a *ArtifactsAPI) ExportArtifact(ctx context.Context, notebookID, artifactID string, exportType rpc.ExportType) (string, error) {
	params := []any{notebookID, artifactID, int(exportType)}
	result, err := a.client.RPCCall(ctx, rpc.ExportArtifact, params, "/notebook/"+notebookID, false)
	if err != nil {
		return "", fmt.Errorf("export artifact: %w", err)
	}
	arr, _ := result.([]any)
	if len(arr) > 0 {
		if u, ok := arr[0].(string); ok {
			return u, nil
		}
	}
	return "", nil
}

// ShareArtifact creates a shareable link for an artifact.
func (a *ArtifactsAPI) ShareArtifact(ctx context.Context, notebookID, artifactID string) (string, error) {
	params := []any{notebookID, artifactID}
	result, err := a.client.RPCCall(ctx, rpc.ShareArtifact, params, "/notebook/"+notebookID, false)
	if err != nil {
		return "", fmt.Errorf("share artifact: %w", err)
	}
	arr, _ := result.([]any)
	if len(arr) > 0 {
		if u, ok := arr[0].(string); ok {
			return u, nil
		}
	}
	return "", nil
}

// GetInteractiveHTML returns the interactive HTML content for an artifact.
func (a *ArtifactsAPI) GetInteractiveHTML(ctx context.Context, notebookID, artifactID string) (string, error) {
	params := []any{notebookID, artifactID}
	result, err := a.client.RPCCall(ctx, rpc.GetInteractiveHTML, params, "/notebook/"+notebookID, false)
	if err != nil {
		return "", fmt.Errorf("get interactive HTML: %w", err)
	}
	arr, _ := result.([]any)
	if len(arr) > 0 {
		if html, ok := arr[0].(string); ok {
			return html, nil
		}
	}
	return "", nil
}

// ReviseSlide revises a single slide in a slide deck artifact using natural language instructions.
// NOTE: slideIndex is 0-based. The exact parameter structure is best-effort based on reverse engineering.
func (a *ArtifactsAPI) ReviseSlide(ctx context.Context, notebookID, artifactID string, slideIndex int, instructions string) (*GenerationStatus, error) {
	params := []any{notebookID, artifactID, slideIndex, instructions}
	result, err := a.client.RPCCall(ctx, rpc.ReviseSlide, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("revise slide: %w", err)
	}
	arr, _ := result.([]any)
	taskID := ""
	if len(arr) > 0 {
		if id, ok := arr[0].(string); ok {
			taskID = id
		}
	}
	return &GenerationStatus{
		TaskID: taskID,
		Status: "pending",
	}, nil
}

func (a *ArtifactsAPI) createArtifact(ctx context.Context, notebookID string, params []any) (*GenerationStatus, error) {
	result, err := a.client.RPCCall(ctx, rpc.CreateArtifact, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}

	// Extract task ID from response
	arr, _ := result.([]any)
	taskID := ""
	if len(arr) > 0 {
		if id, ok := arr[0].(string); ok {
			taskID = id
		}
	}

	return &GenerationStatus{
		TaskID: taskID,
		Status: "pending",
	}, nil
}

func buildSourceParam(sourceIDs []string) any {
	if len(sourceIDs) == 0 {
		return nil
	}
	nested := make([]any, len(sourceIDs))
	for i, id := range sourceIDs {
		nested[i] = []any{id}
	}
	return []any{[]any{nested}}
}

// buildArtifactParams constructs the nested array parameter for CREATE_ARTIFACT.
// Position [2][2]: ArtifactTypeCode
// Position [2][3]: Source IDs
// Additional positions for format config
func buildArtifactParams(notebookID string, typeCode rpc.ArtifactTypeCode, sourceParam any, lang, instructions string, formatConfig []any) []any {
	artifactConfig := []any{nil, nil, int(typeCode), sourceParam}
	if instructions != "" {
		artifactConfig = append(artifactConfig, instructions)
	}
	if formatConfig != nil {
		if instructions == "" {
			artifactConfig = append(artifactConfig, nil)
		}
		artifactConfig = append(artifactConfig, formatConfig)
	}

	return []any{
		notebookID,
		[]any{2},
		artifactConfig,
		lang,
	}
}
