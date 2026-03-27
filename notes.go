package notebooklm

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
)

// NotesAPI provides note and mind map operations.
type NotesAPI struct {
	client *Client
}

// List returns all notes in a notebook.
func (n *NotesAPI) List(ctx context.Context, notebookID string) ([]Note, error) {
	params := []any{notebookID}
	result, err := n.client.RPCCall(ctx, rpc.GetNotesAndMindMaps, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	return parseNotes(result, notebookID), nil
}

// Create creates a new note in a notebook.
func (n *NotesAPI) Create(ctx context.Context, notebookID, title, content string) (*Note, error) {
	params := []any{notebookID, title, content}
	result, err := n.client.RPCCall(ctx, rpc.CreateNote, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}
	arr, _ := result.([]any)
	return parseNote(arr, notebookID), nil
}

// Update modifies the content of an existing note.
func (n *NotesAPI) Update(ctx context.Context, notebookID, noteID, content string) (*Note, error) {
	params := []any{notebookID, noteID, content}
	result, err := n.client.RPCCall(ctx, rpc.UpdateNote, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("update note %s: %w", noteID, err)
	}
	arr, _ := result.([]any)
	return parseNote(arr, notebookID), nil
}

// Delete removes a note from a notebook.
func (n *NotesAPI) Delete(ctx context.Context, notebookID, noteID string) error {
	params := []any{notebookID, noteID}
	_, err := n.client.RPCCall(ctx, rpc.DeleteNote, params, "/notebook/"+notebookID, true)
	return err
}

// ListMindMaps returns mind map data from the notes system.
func (n *NotesAPI) ListMindMaps(ctx context.Context, notebookID string) ([]any, error) {
	params := []any{notebookID}
	result, err := n.client.RPCCall(ctx, rpc.GetNotesAndMindMaps, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("list mind maps: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	// Mind maps are typically at a different index than notes
	arr, _ := result.([]any)
	if len(arr) > 1 {
		if mindMaps, ok := arr[1].([]any); ok {
			return mindMaps, nil
		}
	}
	return nil, nil
}

// GenerateMindMap generates a mind map from notebook sources.
func (n *NotesAPI) GenerateMindMap(ctx context.Context, notebookID string, sourceIDs []string) (*GenerationStatus, error) {
	sourceParam := buildSourceParam(sourceIDs)
	params := []any{notebookID, sourceParam}
	result, err := n.client.RPCCall(ctx, rpc.GenerateMindMap, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("generate mind map: %w", err)
	}

	arr, _ := result.([]any)
	taskID := ""
	if len(arr) > 0 {
		if id, ok := arr[0].(string); ok {
			taskID = id
		}
	}
	return &GenerationStatus{TaskID: taskID, Status: "pending"}, nil
}

func parseNotes(result any, notebookID string) []Note {
	arr, _ := result.([]any)
	if len(arr) == 0 {
		return nil
	}
	notesRaw, _ := arr[0].([]any)
	notes := make([]Note, 0, len(notesRaw))
	for _, item := range notesRaw {
		if data, ok := item.([]any); ok {
			if note := parseNote(data, notebookID); note != nil {
				notes = append(notes, *note)
			}
		}
	}
	return notes
}

func parseNote(data []any, notebookID string) *Note {
	if len(data) == 0 {
		return nil
	}
	note := &Note{NotebookID: notebookID}
	if id, ok := data[0].(string); ok {
		note.ID = id
	}
	if len(data) > 1 {
		if title, ok := data[1].(string); ok {
			note.Title = title
		}
	}
	if len(data) > 2 {
		if content, ok := data[2].(string); ok {
			note.Content = content
		}
	}
	return note
}
