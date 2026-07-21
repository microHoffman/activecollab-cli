package cli

import (
	"github.com/spf13/cobra"
)

func newProjectCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"project",
		"Read ActiveCollab projects",
		"List projects visible to the configured account or read one project by ID.",
	)
	command.AddCommand(
		&cobra.Command{
			Use:               "list",
			Short:             "List accessible projects",
			Long:              "List every ActiveCollab project visible to the configured account.",
			Example:           "  activecollab project list\n  activecollab project list --json",
			Args:              cobra.NoArgs,
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, _ []string) error {
				client, err := options.client()
				if err != nil {
					return err
				}
				projects, err := client.ListProjects(cmd.Context())
				if err != nil {
					return err
				}
				return writeOutput(options.json, projects)
			},
		},
		&cobra.Command{
			Use:               "get <project-id>",
			Short:             "Read a project",
			Long:              "Read one ActiveCollab project by its positive numeric ID.",
			Example:           "  activecollab project get 7\n  activecollab project get 7 --json",
			Args:              cobra.ExactArgs(1),
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, args []string) error {
				id, err := parsePositiveID("project ID", args[0])
				if err != nil {
					return err
				}
				client, err := options.client()
				if err != nil {
					return err
				}
				project, err := client.GetProject(cmd.Context(), id)
				if err != nil {
					return err
				}
				return writeOutput(options.json, project)
			},
		},
	)
	return command
}

func newUserCommand(options *rootOptions) *cobra.Command {
	command := newCommandGroup(
		"user",
		"Read ActiveCollab users",
		"List users visible to the configured account or read one user by ID.",
	)
	command.AddCommand(
		&cobra.Command{
			Use:               "list",
			Short:             "List users",
			Long:              "List ActiveCollab users visible to the configured account.",
			Example:           "  activecollab user list\n  activecollab user list --json",
			Args:              cobra.NoArgs,
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, _ []string) error {
				client, err := options.client()
				if err != nil {
					return err
				}
				users, err := client.ListUsers(cmd.Context())
				if err != nil {
					return err
				}
				return writeOutput(options.json, users)
			},
		},
		&cobra.Command{
			Use:               "get <user-id>",
			Short:             "Read a user",
			Long:              "Read one ActiveCollab user by their positive numeric ID.",
			Example:           "  activecollab user get 9\n  activecollab user get 9 --json",
			Args:              cobra.ExactArgs(1),
			ValidArgsFunction: cobra.NoFileCompletions,
			RunE: func(cmd *cobra.Command, args []string) error {
				id, err := parsePositiveID("user ID", args[0])
				if err != nil {
					return err
				}
				client, err := options.client()
				if err != nil {
					return err
				}
				user, err := client.GetUser(cmd.Context(), id)
				if err != nil {
					return err
				}
				return writeOutput(options.json, user)
			},
		},
	)
	return command
}

func newTaskListCommand(options *rootOptions) *cobra.Command {
	var projectID int
	list := &cobra.Command{
		Use:               "list",
		Short:             "List task lists in a project",
		Long:              "List the task lists configured in one ActiveCollab project.",
		Example:           "  activecollab task-list list --project 7\n  activecollab task-list list --project 7 --json",
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
			lists, err := client.ListTaskLists(cmd.Context(), projectID)
			if err != nil {
				return err
			}
			return writeOutput(options.json, lists)
		},
	}
	list.Flags().IntVar(&projectID, "project", 0, "project ID (required)")
	markFlagRequired(list, "project")
	command := newCommandGroup(
		"task-list",
		"Read ActiveCollab task lists",
		"Read the task-list structure configured within an ActiveCollab project.",
	)
	command.AddCommand(list)
	return command
}
