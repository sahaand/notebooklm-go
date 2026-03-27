// Command notebooklm provides a CLI for interacting with Google NotebookLM.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
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

// --------------------------------------------------------------------------
// Notebook commands
// --------------------------------------------------------------------------

var notebookCmd = &cobra.Command{
	Use:   "notebook",
	Short: "Manage notebooks",
}

func init() {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all notebooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			notebooks, err := c.Notebooks.List(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(notebooks)
				return nil
			}
			if len(notebooks) == 0 {
				fmt.Println("No notebooks found.")
				return nil
			}
			for _, nb := range notebooks {
				fmt.Printf("%s\t%s\n", nb.ID, nb.Title)
			}
			return nil
		},
	}

	createCmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb, err := c.Notebooks.Create(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(nb)
				return nil
			}
			fmt.Printf("Created notebook: %s (%s)\n", nb.Title, nb.ID)
			return nil
		},
	}

	getCmd := &cobra.Command{
		Use:   "get <notebook-id>",
		Short: "Get notebook details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb, err := c.Notebooks.Get(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(nb)
				return nil
			}
			fmt.Printf("ID:      %s\nTitle:   %s\nSources: %d\n", nb.ID, nb.Title, nb.SourcesCount)
			return nil
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete <notebook-id>",
		Short: "Delete a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Notebooks.Delete(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted notebook: %s\n", args[0])
			return nil
		},
	}

	renameCmd := &cobra.Command{
		Use:   "rename <notebook-id> <new-title>",
		Short: "Rename a notebook",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb, err := c.Notebooks.Rename(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Renamed to: %s\n", nb.Title)
			return nil
		},
	}

	describeCmd := &cobra.Command{
		Use:   "describe <notebook-id>",
		Short: "Get AI-generated notebook summary and suggested topics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			desc, err := c.Notebooks.GetDescription(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(desc)
				return nil
			}
			fmt.Println("Summary:", desc.Summary)
			if len(desc.SuggestedTopics) > 0 {
				fmt.Println("\nSuggested topics:")
				for _, t := range desc.SuggestedTopics {
					fmt.Printf("  Q: %s\n", t.Question)
				}
			}
			return nil
		},
	}

	notebookCmd.AddCommand(listCmd, createCmd, getCmd, deleteCmd, renameCmd, describeCmd)
}

// --------------------------------------------------------------------------
// Source commands
// --------------------------------------------------------------------------

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Manage notebook sources",
}

func init() {
	var notebookID string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List sources in a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			sources, err := c.Sources.List(context.Background(), notebookID)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(sources)
				return nil
			}
			for _, s := range sources {
				fmt.Printf("%s\t%s\t%s\n", s.ID, string(s.Kind()), s.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	listCmd.MarkFlagRequired("notebook")

	addURLCmd := &cobra.Command{
		Use:   "add-url <url>",
		Short: "Add a URL or YouTube video as a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			src, err := c.Sources.AddURL(context.Background(), notebookID, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Added source: %s (%s)\n", src.ID, args[0])
			return nil
		},
	}
	addURLCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	addURLCmd.MarkFlagRequired("notebook")

	addTextCmd := &cobra.Command{
		Use:   "add-text <title> <content>",
		Short: "Add pasted text as a source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			src, err := c.Sources.AddText(context.Background(), notebookID, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Added text source: %s\n", src.ID)
			return nil
		},
	}
	addTextCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	addTextCmd.MarkFlagRequired("notebook")

	addFileCmd := &cobra.Command{
		Use:   "add-file <path>",
		Short: "Upload and add a file as a source (PDF, TXT, MD, DOCX)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			src, err := c.Sources.AddFile(context.Background(), notebookID, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Added file source: %s\n", src.ID)
			return nil
		},
	}
	addFileCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	addFileCmd.MarkFlagRequired("notebook")

	deleteCmd := &cobra.Command{
		Use:   "delete <source-id>",
		Short: "Delete a source from a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Sources.Delete(context.Background(), notebookID, args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted source: %s\n", args[0])
			return nil
		},
	}
	deleteCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	deleteCmd.MarkFlagRequired("notebook")

	sourceCmd.AddCommand(listCmd, addURLCmd, addTextCmd, addFileCmd, deleteCmd)
}

// --------------------------------------------------------------------------
// Artifact commands
// --------------------------------------------------------------------------

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Generate and manage studio artifacts",
}

func init() {
	var (
		notebookID   string
		language     string
		instructions string
		waitFlag     bool
		destPath     string
		sourceIDs    []string
	)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts in a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			artifacts, err := c.Artifacts.List(context.Background(), notebookID)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(artifacts)
				return nil
			}
			for _, a := range artifacts {
				fmt.Printf("%s\t%s\t%s\t%s\n", a.ID, string(a.Kind()), rpc.ArtifactStatusToStr(a.Status), a.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	listCmd.MarkFlagRequired("notebook")

	audioCmd := &cobra.Command{
		Use:   "audio",
		Short: "Generate an audio overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			opts := notebooklm.GenerateAudioOptions{
				SourceIDs:    sourceIDs,
				Language:     language,
				Instructions: instructions,
			}
			status, err := c.Artifacts.GenerateAudio(context.Background(), notebookID, opts)
			if err != nil {
				return err
			}
			fmt.Printf("Started audio generation: task=%s\n", status.TaskID)

			if waitFlag && status.TaskID != "" {
				fmt.Println("Waiting for completion...")
				status, err = c.Artifacts.WaitForCompletion(context.Background(), notebookID, status.TaskID, 10*time.Minute)
				if err != nil {
					return err
				}
				fmt.Printf("Status: %s\n", status.Status)
				if destPath != "" && status.URL != "" {
					if err := c.Artifacts.Download(context.Background(), notebookID, status.TaskID, destPath); err != nil {
						return fmt.Errorf("download: %w", err)
					}
					fmt.Printf("Downloaded to: %s\n", destPath)
				}
			}
			return nil
		},
	}
	audioCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	audioCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	audioCmd.Flags().StringVar(&instructions, "instructions", "", "custom instructions")
	audioCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	audioCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	audioCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include (repeat for multiple)")
	audioCmd.MarkFlagRequired("notebook")

	downloadCmd := &cobra.Command{
		Use:   "download <artifact-id> <dest-path>",
		Short: "Download a completed artifact",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Artifacts.Download(context.Background(), notebookID, args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Downloaded to: %s\n", args[1])
			return nil
		},
	}
	downloadCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	downloadCmd.MarkFlagRequired("notebook")

	deleteCmd := &cobra.Command{
		Use:   "delete <artifact-id>",
		Short: "Delete an artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			if err := c.Artifacts.Delete(context.Background(), notebookID, args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted artifact: %s\n", args[0])
			return nil
		},
	}
	deleteCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	deleteCmd.MarkFlagRequired("notebook")

	artifactCmd.AddCommand(listCmd, audioCmd, downloadCmd, deleteCmd)
}

// --------------------------------------------------------------------------
// Chat commands
// --------------------------------------------------------------------------

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat with notebook sources",
}

func init() {
	var (
		notebookID string
		sourceIDs  []string
	)

	askCmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Ask a question about notebook sources",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			question := strings.Join(args, " ")
			opts := notebooklm.AskOptions{SourceIDs: sourceIDs}
			result, err := c.Chat.Ask(context.Background(), notebookID, question, opts)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(result)
				return nil
			}
			fmt.Println(result.Answer)
			return nil
		},
	}
	askCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	askCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "limit to these source IDs")
	askCmd.MarkFlagRequired("notebook")

	chatCmd.AddCommand(askCmd)
}

// --------------------------------------------------------------------------
// Settings commands
// --------------------------------------------------------------------------

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage user settings",
}

func init() {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get current user settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			settings, err := c.Settings.Get(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(settings)
				return nil
			}
			fmt.Printf("Output language: %s\n", settings.OutputLanguage)
			return nil
		},
	}

	setLangCmd := &cobra.Command{
		Use:   "set-language <language-code>",
		Short: "Set output language (e.g., en, es, fr, de, ja)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			settings, err := c.Settings.SetOutputLanguage(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Output language set to: %s\n", settings.OutputLanguage)
			return nil
		},
	}

	settingsCmd.AddCommand(getCmd, setLangCmd)
}
