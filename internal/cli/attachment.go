package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

func newAttachmentCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"attachment",
		"List and download task and comment attachments",
		"List attachment metadata from a task and its comments, or download one owned attachment safely.",
	)
	command.AddCommand(
		newAttachmentListSubcommand(options),
		newAttachmentDownloadSubcommand(options),
	)
	return command
}

func newAttachmentListSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	command := &cobra.Command{
		Use:   "list <task-id-or-url>",
		Short: "List task and comment attachments",
		Long: `List attachment metadata from a task and all of its comments. Provide a full
task URL, or a numeric task ID together with --project.`,
		Example: `  activecollab attachment list 22 --project 7
  activecollab attachment list https://activecollab.example.com/projects/7/tasks/22 --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			attachments, err := client.ListAttachments(cmd.Context(), ref)
			if err != nil {
				return err
			}
			return writeOutput(options.json, attachments)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	return command
}

func newAttachmentDownloadSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	var output string
	var force bool
	command := &cobra.Command{
		Use:   "download <task-id-or-url> <attachment-id>",
		Short: "Download an owned attachment safely",
		Long: `Download an attachment only after verifying that it belongs to the supplied
task or one of its comments. The completed download is published atomically.
Existing files are preserved unless --force is explicitly provided.`,
		Example: `  activecollab attachment download 22 31 --project 7 --output ./spec.txt --dry-run
  activecollab attachment download https://activecollab.example.com/projects/7/tasks/22 31 \
    --output ./spec.txt`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			attachmentID, err := parsePositiveID("attachment ID", args[1])
			if err != nil {
				return err
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			if output == "" {
				return fmt.Errorf("--output is required")
			}
			if options.dryRun {
				return dryRunOutput(options, "attachment.download", map[string]any{
					"task": ref, "attachment_id": attachmentID, "output": output, "force": force,
				})
			}
			attachments, err := client.ListAttachments(cmd.Context(), ref)
			if err != nil {
				return err
			}
			var selectedAttachment activecollab.Attachment
			for _, attachment := range attachments {
				if attachment.ID == attachmentID {
					selectedAttachment = attachment
					break
				}
			}
			if selectedAttachment.ID == 0 {
				return fmt.Errorf("attachment %d does not belong to task %d", attachmentID, ref.TaskID)
			}
			if _, err := os.Stat(output); err == nil && !force {
				return fmt.Errorf("output file %q already exists; use --force to replace it", output)
			} else if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("inspect output file: %w", err)
			}
			directory := filepath.Dir(output)
			temporary, err := os.CreateTemp(directory, ".activecollab-download-*")
			if err != nil {
				return fmt.Errorf("create temporary download file: %w", err)
			}
			temporaryName := temporary.Name()
			cleanup := func() {
				_ = temporary.Close()
				_ = os.Remove(temporaryName)
			}
			result, err := client.DownloadAttachment(cmd.Context(), selectedAttachment, temporary)
			if err != nil {
				cleanup()
				return err
			}
			if err := temporary.Sync(); err != nil {
				cleanup()
				return fmt.Errorf("sync downloaded attachment: %w", err)
			}
			if err := temporary.Close(); err != nil {
				cleanup()
				return fmt.Errorf("close downloaded attachment: %w", err)
			}
			if err := commitDownloadedFile(temporaryName, output, force); err != nil {
				cleanup()
				return err
			}
			return writeOutput(options.json, map[string]any{
				"attachment_id": attachmentID,
				"output":        output,
				"size":          result.Size,
				"content_type":  result.ContentType,
			})
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	command.Flags().StringVar(&output, "output", "", "output file path (required)")
	command.Flags().BoolVar(&force, "force", false, "replace an existing output file")
	addDryRunFlag(command, options)
	markFlagRequired(command, "output")
	markFlagFilename(command, "output")
	return command
}

func commitDownloadedFile(temporaryName, output string, force bool) error {
	return commitDownloadedFileWithLink(temporaryName, output, force, os.Link)
}

func commitDownloadedFileWithLink(temporaryName, output string, force bool, link func(string, string) error) error {
	if force {
		if err := replaceDownloadedFile(temporaryName, output); err != nil {
			return fmt.Errorf("atomically replace downloaded attachment: %w", err)
		}
		return nil
	}

	// Both paths are in the same directory, so linking atomically publishes the
	// complete temporary file and fails if another process won the destination.
	if err := link(temporaryName, output); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file %q already exists; use --force to replace it", output)
		}
		// Some removable and network filesystems do not support hard links. An
		// exclusive create still guarantees that a concurrent writer cannot be
		// overwritten, even though publication is no longer a single rename.
		if err := copyDownloadedFileNoReplace(temporaryName, output); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("output file %q already exists; use --force to replace it", output)
			}
			return fmt.Errorf("move downloaded attachment into place without replacing: %w", err)
		}
		return nil
	}
	if err := os.Remove(temporaryName); err != nil {
		return fmt.Errorf("remove temporary download link: %w", err)
	}
	return nil
}

func copyDownloadedFileNoReplace(temporaryName, output string) error {
	source, err := os.Open(temporaryName)
	if err != nil {
		return fmt.Errorf("open temporary download: %w", err)
	}
	sourceOpen := true
	defer func() {
		if sourceOpen {
			_ = source.Close()
		}
	}()

	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect temporary download: %w", err)
	}
	destination, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create output file exclusively: %w", err)
	}
	removeIncomplete := true
	defer func() {
		_ = destination.Close()
		if removeIncomplete {
			_ = os.Remove(output)
		}
	}()

	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy downloaded attachment: %w", err)
	}
	if err := destination.Sync(); err != nil {
		return fmt.Errorf("sync downloaded attachment: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close downloaded attachment: %w", err)
	}
	removeIncomplete = false
	sourceErr := source.Close()
	sourceOpen = false
	if sourceErr != nil {
		return fmt.Errorf("close temporary download: %w", sourceErr)
	}
	if err := os.Remove(temporaryName); err != nil {
		return fmt.Errorf("remove temporary download file: %w", err)
	}
	return nil
}
