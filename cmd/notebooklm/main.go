// Command notebooklm provides a CLI for interacting with Google NotebookLM.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	notebooklm "github.com/saeedata/notebooklm-go"
)

var (
	storagePath string
	outputJSON  bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "notebooklm",
	Short: "CLI for Google NotebookLM — unofficial Go client",
	Long: `notebooklm is an unofficial CLI for Google NotebookLM.
It uses reverse-engineered APIs to provide programmatic access to notebooks,
sources, audio overviews, and more.

Authentication: run 'notebooklm login' first (requires a Playwright-exported
storage state file, or set NOTEBOOKLM_AUTH_JSON env var).`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&storagePath, "storage", "", "path to Playwright storage_state.json (default: ~/.notebooklm/storage_state.json)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output results as JSON")

	rootCmd.AddCommand(
		loginCmd,
		notebookCmd,
		sourceCmd,
		artifactCmd,
		chatCmd,
		notesCmd,
		shareCmd,
		researchCmd,
		settingsCmd,
	)
}

// client creates an authenticated client.
func client() (*notebooklm.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return notebooklm.NewClientFromStorage(ctx, storagePath)
}

// printJSON outputs v as formatted JSON.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// --------------------------------------------------------------------------
// Context (default notebook) helpers
// --------------------------------------------------------------------------

type notebookContext struct {
	NotebookID string `json:"notebook_id"`
}

func contextFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".notebooklm", "context.json")
}

// readContextNotebook returns the default notebook ID from the context file,
// or empty string if none is set.
func readContextNotebook() string {
	data, err := os.ReadFile(contextFilePath())
	if err != nil {
		return ""
	}
	var ctx notebookContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return ""
	}
	return ctx.NotebookID
}

// writeContextNotebook saves a default notebook ID to the context file.
func writeContextNotebook(id string) error {
	path := contextFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(notebookContext{NotebookID: id}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// clearContextNotebook removes the context file.
func clearContextNotebook() error {
	err := os.Remove(contextFilePath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// notebookOrCtx returns the flag value if non-empty, otherwise falls back to
// the saved context notebook ID.
func notebookOrCtx(flag string) string {
	if flag != "" {
		return flag
	}
	return readContextNotebook()
}

// requireNotebook returns an error if id is empty, with a helpful message.
func requireNotebook(id string) error {
	if id == "" {
		return fmt.Errorf("notebook ID required: use -n <id> or run 'notebooklm notebook use <id>'")
	}
	return nil
}

// --------------------------------------------------------------------------
// Login command
// --------------------------------------------------------------------------

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google NotebookLM",
	Long: `Authenticate using an exported Playwright storage state.

To create the storage state file:
  1. Install Playwright: npm install -g playwright && playwright install chromium
  2. Log in to notebooklm.google.com in a browser
  3. Export storage: playwright storage export ~/.notebooklm/storage_state.json

Alternatively, set NOTEBOOKLM_AUTH_JSON to the JSON content directly.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		_, err := notebooklm.NewClientFromStorage(ctx, storagePath)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		fmt.Println("Authentication successful.")
		return nil
	},
}

