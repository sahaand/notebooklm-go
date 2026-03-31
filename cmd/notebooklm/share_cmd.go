package main

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
	"github.com/spf13/cobra"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Manage notebook sharing",
}

func init() {
	var (
		notebookID string
		public     bool
		permission string
		notify     bool
		message    string
	)

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Get sharing status for a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			s, err := c.Sharing.GetStatus(context.Background(), nb)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(s)
				return nil
			}
			fmt.Printf("Public:    %v\n", s.IsPublic)
			if s.ShareURL != "" {
				fmt.Printf("Share URL: %s\n", s.ShareURL)
			}
			if len(s.SharedUsers) > 0 {
				fmt.Println("\nShared with:")
				for _, u := range s.SharedUsers {
					perm := "viewer"
					if u.Permission == rpc.ShareEditor {
						perm = "editor"
					} else if u.Permission == rpc.ShareOwner {
						perm = "owner"
					}
					fmt.Printf("  %s (%s)\n", u.Email, perm)
				}
			}
			return nil
		},
	}
	statusCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	setPublicCmd := &cobra.Command{
		Use:   "set-public",
		Short: "Enable or disable public link sharing",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			s, err := c.Sharing.SetPublic(context.Background(), nb, public)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(s)
				return nil
			}
			fmt.Printf("Public sharing: %v\n", s.IsPublic)
			if s.ShareURL != "" {
				fmt.Printf("Share URL: %s\n", s.ShareURL)
			}
			return nil
		},
	}
	setPublicCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	setPublicCmd.Flags().BoolVar(&public, "public", false, "enable public link sharing")

	addUserCmd := &cobra.Command{
		Use:   "add-user <email>",
		Short: "Share notebook with a user by email",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			perm := rpc.ShareViewer
			if permission == "editor" {
				perm = rpc.ShareEditor
			}
			s, err := c.Sharing.AddUser(context.Background(), nb, args[0], perm, notify, message)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(s)
				return nil
			}
			fmt.Printf("Added %s as %s\n", args[0], permission)
			return nil
		},
	}
	addUserCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	addUserCmd.Flags().StringVar(&permission, "permission", "viewer", "permission: viewer, editor")
	addUserCmd.Flags().BoolVar(&notify, "notify", false, "send email notification")
	addUserCmd.Flags().StringVar(&message, "message", "", "optional message for notification email")

	updateUserCmd := &cobra.Command{
		Use:   "update-user <email>",
		Short: "Update a user's permission level",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			perm := rpc.ShareViewer
			if permission == "editor" {
				perm = rpc.ShareEditor
			}
			s, err := c.Sharing.UpdateUser(context.Background(), nb, args[0], perm)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(s)
				return nil
			}
			fmt.Printf("Updated %s to %s\n", args[0], permission)
			return nil
		},
	}
	updateUserCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	updateUserCmd.Flags().StringVar(&permission, "permission", "viewer", "permission: viewer, editor")
	updateUserCmd.MarkFlagRequired("permission")

	removeUserCmd := &cobra.Command{
		Use:   "remove-user <email>",
		Short: "Remove a user's access to the notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			s, err := c.Sharing.RemoveUser(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(s)
				return nil
			}
			fmt.Printf("Removed user: %s\n", args[0])
			return nil
		},
	}
	removeUserCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	shareCmd.AddCommand(statusCmd, setPublicCmd, addUserCmd, updateUserCmd, removeUserCmd)
}
