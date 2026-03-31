package main

import (
	"context"
	"fmt"
	"strings"

	notebooklm "github.com/saeedata/notebooklm-go"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat with notebook sources",
}

func init() {
	var (
		notebookID     string
		sourceIDs      []string
		conversationID string
		limit          int
		noteTitle      string
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
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			question := strings.Join(args, " ")
			opts := notebooklm.AskOptions{SourceIDs: sourceIDs}
			result, err := c.Chat.Ask(context.Background(), nb, question, opts)
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

	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Get conversation history for a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			convID := conversationID
			if convID == "" {
				id, err := c.Chat.GetLastConversationID(context.Background(), nb)
				if err != nil {
					return err
				}
				if id == "" {
					fmt.Println("No conversation found.")
					return nil
				}
				convID = id
			}
			if limit <= 0 {
				limit = 10
			}
			turns, err := c.Chat.GetConversationTurns(context.Background(), nb, convID, limit)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(turns)
				return nil
			}
			for _, t := range turns {
				fmt.Printf("Q: %s\nA: %s\n\n", t.Query, t.Answer)
			}
			return nil
		},
	}
	historyCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	historyCmd.Flags().StringVar(&conversationID, "conversation-id", "", "conversation ID (default: last conversation)")
	historyCmd.Flags().IntVar(&limit, "limit", 10, "max number of turns to return")

	saveToNotesCmd := &cobra.Command{
		Use:   "save-to-notes [turn-index]",
		Short: "Save a conversation turn as a notebook note",
		Long:  "Fetches the last conversation and saves the specified turn (0-based, default: last) as a note.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			convID := conversationID
			if convID == "" {
				id, err := c.Chat.GetLastConversationID(context.Background(), nb)
				if err != nil {
					return err
				}
				if id == "" {
					return fmt.Errorf("no conversation found in notebook %s", nb)
				}
				convID = id
			}
			turns, err := c.Chat.GetConversationTurns(context.Background(), nb, convID, 50)
			if err != nil {
				return err
			}
			if len(turns) == 0 {
				return fmt.Errorf("no conversation turns found")
			}
			// Select turn: last by default, or by index if provided
			turn := turns[len(turns)-1]
			if len(args) == 1 {
				idx := 0
				if _, err := fmt.Sscanf(args[0], "%d", &idx); err != nil {
					return fmt.Errorf("turn-index must be an integer")
				}
				if idx < 0 || idx >= len(turns) {
					return fmt.Errorf("turn-index %d out of range (0-%d)", idx, len(turns)-1)
				}
				turn = turns[idx]
			}
			title := noteTitle
			if title == "" {
				title = turn.Query
				if len(title) > 60 {
					title = title[:60] + "..."
				}
			}
			content := fmt.Sprintf("**Q:** %s\n\n**A:** %s", turn.Query, turn.Answer)
			note, err := c.Notes.Create(context.Background(), nb, title, content)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(note)
				return nil
			}
			fmt.Printf("Saved as note: %s (%s)\n", note.Title, note.ID)
			return nil
		},
	}
	saveToNotesCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID")
	saveToNotesCmd.Flags().StringVar(&conversationID, "conversation-id", "", "conversation ID (default: last conversation)")
	saveToNotesCmd.Flags().StringVar(&noteTitle, "title", "", "note title (default: question text)")

	chatCmd.AddCommand(askCmd, historyCmd, saveToNotesCmd)
}
