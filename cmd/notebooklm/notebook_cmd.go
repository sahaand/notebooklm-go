package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

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
				fmt.Printf("%-60s  %s\n", nb.Title, nb.ID)
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

	metadataCmd := &cobra.Command{
		Use:   "metadata <notebook-id>",
		Short: "Get notebook metadata with source list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			meta, err := c.Notebooks.GetMetadata(context.Background(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(meta)
				return nil
			}
			fmt.Printf("ID:      %s\nTitle:   %s\n", meta.Notebook.ID, meta.Notebook.Title)
			if len(meta.Sources) > 0 {
				fmt.Println("\nSources:")
				for _, s := range meta.Sources {
					if s.URL != "" {
						fmt.Printf("  [%s] %s  (%s)\n", s.Kind, s.Title, s.URL)
					} else {
						fmt.Printf("  [%s] %s\n", s.Kind, s.Title)
					}
				}
			}
			return nil
		},
	}

	notebookCmd.AddCommand(listCmd, createCmd, getCmd, deleteCmd, renameCmd, describeCmd, metadataCmd)
}
