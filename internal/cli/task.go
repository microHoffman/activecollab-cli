package cli

import (
	"fmt"
	"os"
	"strings"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

func newTaskCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"task",
		"Create, read, and update ActiveCollab tasks",
		"List, inspect, create, update, complete, and reopen ActiveCollab tasks.",
	)
	command.AddCommand(
		newTaskListSubcommand(options),
		newTaskGetSubcommand(options),
		newTaskCreateSubcommand(options),
		newTaskUpdateSubcommand(options),
		newTaskStateSubcommand(options, true),
		newTaskStateSubcommand(options, false),
		newTaskHistorySubcommand(options),
	)
	return command
}

func newTaskListSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	command := &cobra.Command{
		Use:               "list",
		Short:             "List tasks in a project",
		Long:              "List tasks in one ActiveCollab project.",
		Example:           "  activecollab task list --project 7\n  activecollab task list --project 7 --json",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requirePositive("project ID", projectID); err != nil {
				return err
			}
			client, err := options.client()
			if err != nil {
				return err
			}
			tasks, err := client.ListTasks(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			return writeOutput(options.json, tasks)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID (required)")
	markFlagRequired(command, "project")
	return command
}

func newTaskGetSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	command := &cobra.Command{
		Use:   "get <task-id-or-url>",
		Short: "Read a task with comments, subtasks, and attachments",
		Long: `Read a task and its embedded comments, subtasks, and attachment metadata.

Provide a full task URL, or provide a numeric task ID together with --project.`,
		Example: `  activecollab task get https://activecollab.example.com/projects/7/tasks/22
  activecollab task get 22 --project 7 --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			task, err := client.GetTask(cmd.Context(), ref)
			if err != nil {
				return err
			}
			return writeOutput(options.json, task)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	return command
}

func newTaskCreateSubcommand(options *rootOptions) *cobra.Command {
	var projectID, assigneeID, taskListID int
	var name, body, bodyFile, dueOn string
	var important bool
	var attachments []string
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a task",
		Long: `Create a task in one project. Use --body-file for multiline content and
repeat --attach for each file. Files are uploaded before the task is created;
the task is not created if an upload fails.`,
		Example: `  activecollab task create --project 7 --name "Add contract tests" --dry-run
  activecollab task create --project 7 --name "Add contract tests" \
    --body-file task-description.md --assignee-id 9 --attach specification.txt`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requirePositive("project ID", projectID); err != nil {
				return err
			}
			if err := validateChangedPositive(cmd, "assignee-id", "assignee ID", assigneeID); err != nil {
				return err
			}
			if err := validateChangedPositive(cmd, "task-list-id", "task list ID", taskListID); err != nil {
				return err
			}
			resolvedBody, err := readBody(body, bodyFile, os.Stdin)
			if err != nil {
				return err
			}
			if err := validateDueDate(cmd, "due-on", dueOn); err != nil {
				return err
			}
			if err := validateAttachments(attachments); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("task name is required")
			}
			input := activecollab.TaskCreateInput{
				Name:        name,
				Body:        changedString(cmd, "body", resolvedBody),
				AssigneeID:  changedInt(cmd, "assignee-id", assigneeID),
				DueOn:       changedString(cmd, "due-on", dueOn),
				TaskListID:  changedInt(cmd, "task-list-id", taskListID),
				IsImportant: changedBool(cmd, "important", important),
				Attachments: attachments,
			}
			if bodyFile != "" {
				input.Body = &resolvedBody
			}
			if options.dryRun {
				if _, err := options.client(); err != nil {
					return err
				}
				return dryRunOutput(options, "task.create", map[string]any{"project_id": projectID, "input": input})
			}
			client, err := options.client()
			if err != nil {
				return err
			}
			task, err := client.CreateTask(cmd.Context(), projectID, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, task)
		},
	}
	flags := command.Flags()
	flags.IntVar(&projectID, "project", 0, "project ID (required)")
	flags.StringVar(&name, "name", "", "task name (required)")
	flags.StringVar(&body, "body", "", "task body")
	flags.StringVar(&bodyFile, "body-file", "", "read task body from a file, or - for stdin")
	flags.IntVar(&assigneeID, "assignee-id", 0, "assignee user ID")
	flags.StringVar(&dueOn, "due-on", "", "due date in YYYY-MM-DD format")
	flags.IntVar(&taskListID, "task-list-id", 0, "task list ID")
	flags.BoolVar(&important, "important", false, "mark the task as important")
	flags.StringArrayVar(&attachments, "attach", nil, "attachment path (repeatable)")
	addDryRunFlag(command, options)
	markFlagRequired(command, "project")
	markFlagRequired(command, "name")
	command.MarkFlagsMutuallyExclusive("body", "body-file")
	markFlagFilename(command, "body-file")
	markFlagFilename(command, "attach")
	return command
}

func newTaskUpdateSubcommand(options *rootOptions) *cobra.Command {
	var projectID, assigneeID, taskListID int
	var name, body, bodyFile, dueOn string
	var important, clearAssignee, clearDueOn, clearTaskList bool
	var attachments []string
	command := &cobra.Command{
		Use:   "update <task-id-or-url>",
		Short: "Update selected task fields",
		Long: `Update only the task fields named by flags. Clearing flags are mutually
exclusive with their corresponding value flags. Provide a full task URL, or a
numeric task ID together with --project.`,
		Example: `  activecollab task update 22 --project 7 --name "Updated name" --dry-run
  activecollab task update https://activecollab.example.com/projects/7/tasks/22 \
    --body-file updated-description.md --clear-assignee`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateChangedPositive(cmd, "assignee-id", "assignee ID", assigneeID); err != nil {
				return err
			}
			if err := validateChangedPositive(cmd, "task-list-id", "task list ID", taskListID); err != nil {
				return err
			}
			if cmd.Flags().Changed("name") && strings.TrimSpace(name) == "" {
				return fmt.Errorf("task name cannot be empty")
			}
			resolvedBody, err := readBody(body, bodyFile, os.Stdin)
			if err != nil {
				return err
			}
			if err := validateDueDate(cmd, "due-on", dueOn); err != nil {
				return err
			}
			if err := validateAttachments(attachments); err != nil {
				return err
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			input := activecollab.TaskUpdateInput{
				Name:          changedString(cmd, "name", name),
				Body:          changedString(cmd, "body", resolvedBody),
				AssigneeID:    changedInt(cmd, "assignee-id", assigneeID),
				ClearAssignee: clearAssignee,
				DueOn:         changedString(cmd, "due-on", dueOn),
				ClearDueOn:    clearDueOn,
				TaskListID:    changedInt(cmd, "task-list-id", taskListID),
				ClearTaskList: clearTaskList,
				IsImportant:   changedBool(cmd, "important", important),
				Attachments:   attachments,
			}
			if bodyFile != "" {
				input.Body = &resolvedBody
			}
			if input.Name == nil && input.Body == nil && input.AssigneeID == nil && !input.ClearAssignee && input.DueOn == nil && !input.ClearDueOn && input.TaskListID == nil && !input.ClearTaskList && input.IsImportant == nil && len(input.Attachments) == 0 {
				return fmt.Errorf("at least one task field must be updated")
			}
			if options.dryRun {
				return dryRunOutput(options, "task.update", map[string]any{"task": ref, "input": input})
			}
			task, err := client.UpdateTask(cmd.Context(), ref, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, task)
		},
	}
	flags := command.Flags()
	flags.IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	flags.StringVar(&name, "name", "", "new task name")
	flags.StringVar(&body, "body", "", "new task body")
	flags.StringVar(&bodyFile, "body-file", "", "read task body from a file, or - for stdin")
	flags.IntVar(&assigneeID, "assignee-id", 0, "new assignee user ID")
	flags.BoolVar(&clearAssignee, "clear-assignee", false, "remove the assignee")
	flags.StringVar(&dueOn, "due-on", "", "new due date in YYYY-MM-DD format")
	flags.BoolVar(&clearDueOn, "clear-due-on", false, "remove the due date")
	flags.IntVar(&taskListID, "task-list-id", 0, "new task list ID")
	flags.BoolVar(&clearTaskList, "clear-task-list", false, "remove the task from its task list")
	flags.BoolVar(&important, "important", false, "set important status; use --important=false to clear")
	flags.StringArrayVar(&attachments, "attach", nil, "attachment path (repeatable)")
	addDryRunFlag(command, options)
	command.MarkFlagsMutuallyExclusive("body", "body-file")
	command.MarkFlagsMutuallyExclusive("assignee-id", "clear-assignee")
	command.MarkFlagsMutuallyExclusive("due-on", "clear-due-on")
	command.MarkFlagsMutuallyExclusive("task-list-id", "clear-task-list")
	command.MarkFlagsOneRequired(
		"name", "body", "body-file", "assignee-id", "clear-assignee", "due-on",
		"clear-due-on", "task-list-id", "clear-task-list", "important", "attach",
	)
	markFlagFilename(command, "body-file")
	markFlagFilename(command, "attach")
	return command
}

func newTaskStateSubcommand(options *rootOptions, complete bool) *cobra.Command {
	var projectID int
	name := "reopen"
	description := "Reopen a completed task"
	operation := "task.reopen"
	if complete {
		name = "complete"
		description = "Complete a task"
		operation = "task.complete"
	}
	long := description + `. The command verifies that the task belongs to the supplied
project before changing its state. Provide a full task URL, or a numeric task ID
together with --project.`
	example := fmt.Sprintf("  activecollab task %s 22 --project 7 --dry-run\n  activecollab task %s https://activecollab.example.com/projects/7/tasks/22", name, name)
	command := &cobra.Command{
		Use:               name + " <task-id-or-url>",
		Short:             description,
		Long:              long,
		Example:           example,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			if options.dryRun {
				return dryRunOutput(options, operation, ref)
			}
			if _, err := client.GetTask(cmd.Context(), ref); err != nil {
				return fmt.Errorf("verify task project membership: %w", err)
			}
			var task activecollab.Task
			if complete {
				task, err = client.CompleteTask(cmd.Context(), ref.TaskID)
			} else {
				task, err = client.ReopenTask(cmd.Context(), ref.TaskID)
			}
			if err != nil {
				return err
			}
			return writeOutput(options.json, task)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	addDryRunFlag(command, options)
	return command
}

func newTaskHistorySubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	var verbose bool
	command := &cobra.Command{
		Use:   "history <task-id-or-url>",
		Short: "Read task change history",
		Long: `Read a task's modification history after verifying its project membership.
Use --verbose to include ActiveCollab's formatted descriptions.`,
		Example: `  activecollab task history 22 --project 7
  activecollab task history https://activecollab.example.com/projects/7/tasks/22 --verbose --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			if _, err := client.GetTask(cmd.Context(), ref); err != nil {
				return fmt.Errorf("verify task project membership: %w", err)
			}
			history, err := client.TaskHistory(cmd.Context(), ref.TaskID, verbose)
			if err != nil {
				return err
			}
			return writeOutput(options.json, history)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	command.Flags().BoolVar(&verbose, "verbose", false, "include ActiveCollab's formatted modification descriptions")
	return command
}
