package notebooklm

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
)

// SettingsAPI provides user settings operations.
type SettingsAPI struct {
	client *Client
}

// UserSettings holds user-level settings.
type UserSettings struct {
	OutputLanguage string `json:"output_language"`
}

// Get returns the current user settings.
func (s *SettingsAPI) Get(ctx context.Context) (*UserSettings, error) {
	params := []any{}
	result, err := s.client.RPCCall(ctx, rpc.GetUserSettings, params, "/", true)
	if err != nil {
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	return parseUserSettings(result), nil
}

// SetOutputLanguage updates the output language for generated content.
// Use BCP 47 language codes (e.g., "en", "es", "fr", "de", "ja").
func (s *SettingsAPI) SetOutputLanguage(ctx context.Context, languageCode string) (*UserSettings, error) {
	params := []any{languageCode}
	_, err := s.client.RPCCall(ctx, rpc.SetUserSettings, params, "/", true)
	if err != nil {
		return nil, fmt.Errorf("set output language to %s: %w", languageCode, err)
	}
	return s.Get(ctx)
}

func parseUserSettings(result any) *UserSettings {
	settings := &UserSettings{OutputLanguage: "en"}
	arr, _ := result.([]any)
	if len(arr) > 0 {
		if lang, ok := arr[0].(string); ok {
			settings.OutputLanguage = lang
		}
	}
	return settings
}
