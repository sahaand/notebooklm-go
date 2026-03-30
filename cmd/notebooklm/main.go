// Command notebooklm provides a CLI for interacting with Google NotebookLM.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

