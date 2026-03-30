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

	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "Get conversation history for a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			convID := conversationID
			if convID == "" {
				id, err := c.Chat.GetLastConversationID(context.Background(), notebookID)
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
			turns, err := c.Chat.GetConversationTurns(context.Background(), notebookID, convID, limit)
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
	historyCmd.MarkFlagRequired("notebook")

	chatCmd.AddCommand(askCmd, historyCmd)
}
