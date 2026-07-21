package cli

import (
	"fmt"
	"strings"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

func newSubtaskCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"subtask",
		"Create, read, and update task subtasks",
		"List, create, update, complete, and reopen subtasks belonging to a task.",
	)
	command.AddCommand(
		newSubtaskListSubcommand(options),
		newSubtaskCreateSubcommand(options),
		newSubtaskUpdateSubcommand(options),
		newSubtaskStateSubcommand(options, true),
		newSubtaskStateSubcommand(options, false),
	)
	return command
}

func newSubtaskListSubcommand(options *rootOptions) *cobra.Command {
	var projectID int
	command := &cobra.Command{
		Use:   "list <task-id-or-url>",
		Short: "List a task's subtasks",
		Long: `List subtasks belonging to a task. Provide a full task URL, or a numeric
task ID together with --project.`,
		Example: `  activecollab subtask list 22 --project 7
  activecollab subtask list https://activecollab.example.com/projects/7/tasks/22 --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			subtasks, err := client.ListSubtasks(cmd.Context(), ref)
			if err != nil {
				return err
			}
			return writeOutput(options.json, subtasks)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	return command
}

func newSubtaskCreateSubcommand(options *rootOptions) *cobra.Command {
	var projectID, assigneeID int
	var name, dueOn string
	command := &cobra.Command{
		Use:   "create <task-id-or-url>",
		Short: "Create a subtask",
		Long: `Create a subtask under an existing task. Provide a full task URL, or a
numeric task ID together with --project.`,
		Example: `  activecollab subtask create 22 --project 7 --name "Add regression test" --dry-run
  activecollab subtask create https://activecollab.example.com/projects/7/tasks/22 \
    --name "Add regression test" --assignee-id 9 --due-on 2026-08-01`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateChangedPositive(cmd, "assignee-id", "assignee ID", assigneeID); err != nil {
				return err
			}
			if err := validateDueDate(cmd, "due-on", dueOn); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("subtask name is required")
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			input := activecollab.SubtaskCreateInput{
				Name:       name,
				AssigneeID: changedInt(cmd, "assignee-id", assigneeID),
				DueOn:      changedString(cmd, "due-on", dueOn),
			}
			if options.dryRun {
				return dryRunOutput(options, "subtask.create", map[string]any{"task": ref, "input": input})
			}
			subtask, err := client.CreateSubtask(cmd.Context(), ref, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, subtask)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	command.Flags().StringVar(&name, "name", "", "subtask name (required)")
	command.Flags().IntVar(&assigneeID, "assignee-id", 0, "assignee user ID")
	command.Flags().StringVar(&dueOn, "due-on", "", "due date in YYYY-MM-DD format")
	addDryRunFlag(command, options)
	markFlagRequired(command, "name")
	return command
}

func newSubtaskUpdateSubcommand(options *rootOptions) *cobra.Command {
	var projectID, assigneeID int
	var name, dueOn string
	var clearAssignee, clearDueOn bool
	command := &cobra.Command{
		Use:   "update <task-id-or-url> <subtask-id>",
		Short: "Update selected subtask fields",
		Long: `Update only the subtask fields named by flags after verifying that the
subtask belongs to the supplied task. Clearing flags are mutually exclusive
with their corresponding value flags.`,
		Example: `  activecollab subtask update 22 51 --project 7 --name "Retest fix" --dry-run
  activecollab subtask update https://activecollab.example.com/projects/7/tasks/22 51 \
    --clear-assignee --due-on 2026-08-01`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateChangedPositive(cmd, "assignee-id", "assignee ID", assigneeID); err != nil {
				return err
			}
			if cmd.Flags().Changed("name") && strings.TrimSpace(name) == "" {
				return fmt.Errorf("subtask name cannot be empty")
			}
			if err := validateDueDate(cmd, "due-on", dueOn); err != nil {
				return err
			}
			subtaskID, err := parsePositiveID("subtask ID", args[1])
			if err != nil {
				return err
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			input := activecollab.SubtaskUpdateInput{
				Name:          changedString(cmd, "name", name),
				AssigneeID:    changedInt(cmd, "assignee-id", assigneeID),
				ClearAssignee: clearAssignee,
				DueOn:         changedString(cmd, "due-on", dueOn),
				ClearDueOn:    clearDueOn,
			}
			if input.Name == nil && input.AssigneeID == nil && !input.ClearAssignee && input.DueOn == nil && !input.ClearDueOn {
				return fmt.Errorf("at least one subtask field must be updated")
			}
			if options.dryRun {
				return dryRunOutput(options, "subtask.update", map[string]any{"task": ref, "subtask_id": subtaskID, "input": input})
			}
			subtask, err := client.UpdateSubtask(cmd.Context(), ref, subtaskID, input)
			if err != nil {
				return err
			}
			return writeOutput(options.json, subtask)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	command.Flags().StringVar(&name, "name", "", "new subtask name")
	command.Flags().IntVar(&assigneeID, "assignee-id", 0, "new assignee user ID")
	command.Flags().BoolVar(&clearAssignee, "clear-assignee", false, "remove the assignee")
	command.Flags().StringVar(&dueOn, "due-on", "", "new due date in YYYY-MM-DD format")
	command.Flags().BoolVar(&clearDueOn, "clear-due-on", false, "remove the due date")
	addDryRunFlag(command, options)
	command.MarkFlagsMutuallyExclusive("assignee-id", "clear-assignee")
	command.MarkFlagsMutuallyExclusive("due-on", "clear-due-on")
	command.MarkFlagsOneRequired("name", "assignee-id", "clear-assignee", "due-on", "clear-due-on")
	return command
}

func newSubtaskStateSubcommand(options *rootOptions, complete bool) *cobra.Command {
	var projectID int
	name := "reopen"
	description := "Reopen a completed subtask"
	operation := "subtask.reopen"
	if complete {
		name = "complete"
		description = "Complete a subtask"
		operation = "subtask.complete"
	}
	long := description + `. The command verifies that the subtask belongs to the
supplied task before changing its state.`
	example := fmt.Sprintf("  activecollab subtask %s 22 51 --project 7 --dry-run\n  activecollab subtask %s https://activecollab.example.com/projects/7/tasks/22 51", name, name)
	command := &cobra.Command{
		Use:               name + " <task-id-or-url> <subtask-id>",
		Short:             description,
		Long:              long,
		Example:           example,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			subtaskID, err := parsePositiveID("subtask ID", args[1])
			if err != nil {
				return err
			}
			client, ref, err := resolveTaskRef(options, args[0], projectID)
			if err != nil {
				return err
			}
			if options.dryRun {
				return dryRunOutput(options, operation, map[string]any{"task": ref, "subtask_id": subtaskID})
			}
			var subtask activecollab.Subtask
			if complete {
				subtask, err = client.CompleteSubtask(cmd.Context(), ref, subtaskID)
			} else {
				subtask, err = client.ReopenSubtask(cmd.Context(), ref, subtaskID)
			}
			if err != nil {
				return err
			}
			return writeOutput(options.json, subtask)
		},
	}
	command.Flags().IntVar(&projectID, "project", 0, "project ID when using a numeric task ID")
	addDryRunFlag(command, options)
	return command
}
