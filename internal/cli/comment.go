package cli

import (
	"fmt"
	"os"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

func newCommentCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"comment",
		"Read and write task comments",
		"List task comments, add a comment to a task, or update a comment body.",
	)
	command.AddCommand(
		newCommentListSubcommand(options),
		newCommentAddSubcommand(options),
		newCommentUpdateSubcommand(options),
	)
	return command
}

func newCommentListSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	command := &cobra.Command{
		Use:   "list <task-id-or-url>",
		Short: "List comments on a task",
		Long: `List comments on a task. Provide a full task URL, or a numeric task ID
together with --project.`,
		Example: `  activecollab comment list 22 --project 7
  activecollab comment list https://activecollab.example.com/projects/7/tasks/22 --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			comments, err := client.ListComments(cmd.Context(), ref)
			if err != nil {
				return err
			}
			return writeOutput(options.json, comments)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	return command
}

func newCommentAddSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	var body, bodyFile string
	var attachments []string
	command := &cobra.Command{
		Use:   "add <task-id-or-url>",
		Short: "Add a comment to a task",
		Long: `Add a nonempty comment to a task. Use --body-file for multiline content and
repeat --attach for each file. The command verifies task membership before
creating the comment.`,
		Example: `  activecollab comment add 22 --project 7 --body "Implemented and tested" --dry-run
  activecollab comment add https://activecollab.example.com/projects/7/tasks/22 \
    --body-file result.md --attach test-output.txt`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedBody, err := readBody(body, bodyFile, os.Stdin)
			if err != nil {
				return err
			}
			if err := requireBody(resolvedBody); err != nil {
				return err
			}
			if err := validateAttachments(attachments); err != nil {
				return err
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			input := activecollab.CommentCreateInput{Body: resolvedBody, Attachments: attachments}
			if options.dryRun {
				return dryRunOutput(options, "comment.add", map[string]any{"task": ref, "input": input})
			}
			if _, err := client.GetTask(cmd.Context(), ref); err != nil {
				return fmt.Errorf("verify task project membership: %w", err)
			}
			comment, err := client.AddComment(cmd.Context(), ref.TaskID, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, comment)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	command.Flags().StringVar(&body, "body", "", "comment body")
	command.Flags().StringVar(&bodyFile, "body-file", "", "read comment body from a file, or - for stdin")
	command.Flags().StringArrayVar(&attachments, "attach", nil, "attachment path (repeatable)")
	addDryRunFlag(command, options)
	command.MarkFlagsOneRequired("body", "body-file")
	command.MarkFlagsMutuallyExclusive("body", "body-file")
	markFlagFilename(command, "body-file")
	markFlagFilename(command, "attach")
	return command
}

func newCommentUpdateSubcommand(options *rootOptions) *cobra.Command {
	var body, bodyFile string
	command := &cobra.Command{
		Use:   "update <comment-id>",
		Short: "Update a comment body",
		Long: `Replace a comment's body by positive numeric comment ID. Use --body-file for
multiline content.`,
		Example: `  activecollab comment update 41 --body "Corrected result" --dry-run
  activecollab comment update 41 --body-file corrected-result.md`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			commentID, err := parsePositiveID("comment ID", args[0])
			if err != nil {
				return err
			}
			resolvedBody, err := readBody(body, bodyFile, os.Stdin)
			if err != nil {
				return err
			}
			if err := requireBody(resolvedBody); err != nil {
				return err
			}
			input := activecollab.CommentUpdateInput{Body: resolvedBody}
			if options.dryRun {
				if _, err := options.client(); err != nil {
					return err
				}
				return dryRunOutput(options, "comment.update", map[string]any{"comment_id": commentID, "input": input})
			}
			client, err := options.client()
			if err != nil {
				return err
			}
			comment, err := client.UpdateComment(cmd.Context(), commentID, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, comment)
		},
	}
	command.Flags().StringVar(&body, "body", "", "new comment body")
	command.Flags().StringVar(&bodyFile, "body-file", "", "read comment body from a file, or - for stdin")
	addDryRunFlag(command, options)
	command.MarkFlagsOneRequired("body", "body-file")
	command.MarkFlagsMutuallyExclusive("body", "body-file")
	markFlagFilename(command, "body-file")
	return command
}
