package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	json         bool
	dryRun       bool
	timeout      time.Duration
	version      string
	stdin        io.Reader
	stderr       io.Writer
	httpClient   *http.Client
	configPath   string
	promptSecret func(string) (string, error)
}

func Execute(version string) int {
	options := newRootOptions(version)
	root := newRootCommand(options)
	if err := root.Execute(); err != nil {
		printError(options.json, err)
		return 1
	}
	return 0
}

// NewCommand builds a complete, side-effect-free command tree. Keeping command
// construction separate from execution lets tests, documentation generators,
// and shell completion inspect exactly the same interface users run.
func NewCommand(version string) *cobra.Command {
	return newRootCommand(newRootOptions(version))
}

func newRootOptions(version string) *rootOptions {
	return &rootOptions{
		timeout: 30 * time.Second,
		version: version,
		stdin:   os.Stdin,
		stderr:  os.Stderr,
	}
}

func newRootCommand(options *rootOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "activecollab",
		Short: "Work with ActiveCollab tasks from the command line",
		Long: `activecollab is an unofficial command-line client for ActiveCollab tasks.

For a self-hosted server, run activecollab auth login and pass its complete
/api/v1 URL. Environment variables remain available for automation. Task
commands accept either a full task URL or a numeric task ID with --project.
Commands that change state provide --dry-run to validate and display the
intended operation without sending it.`,
		Example: `  activecollab auth login --url https://activecollab.example.com/api/v1
  activecollab info
  activecollab task get https://activecollab.example.com/projects/7/tasks/22
  activecollab task update 22 --project 7 --name "Updated name" --dry-run
  activecollab task list --project 7 --json`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	command.AddGroup(
		&cobra.Group{ID: "auth", Title: "Authentication Commands:"},
		&cobra.Group{ID: "discovery", Title: "Discovery Commands:"},
		&cobra.Group{ID: "work", Title: "Work Item Commands:"},
		&cobra.Group{ID: "other", Title: "Other Commands:"},
	)
	command.SetHelpCommandGroupID("other")
	command.SetCompletionCommandGroupID("other")
	command.PersistentFlags().BoolVar(&options.json, "json", false, "emit stable JSON output")
	command.PersistentFlags().DurationVar(&options.timeout, "timeout", options.timeout, "HTTP request timeout")
	authCommand := newAuthCommand(options)
	authCommand.GroupID = "auth"

	discoveryCommands := []*cobra.Command{
		newInfoCommand(options),
		newProjectCommand(options),
		newUserCommand(options),
		newTaskListCommand(options),
	}
	for _, child := range discoveryCommands {
		child.GroupID = "discovery"
	}
	workCommands := []*cobra.Command{
		newTaskCommand(options),
		newCommentCommand(options),
		newSubtaskCommand(options),
		newAttachmentCommand(options),
	}
	for _, child := range workCommands {
		child.GroupID = "work"
	}
	versionCommand := newVersionCommand(options)
	versionCommand.GroupID = "other"
	children := []*cobra.Command{authCommand}
	children = append(children, discoveryCommands...)
	children = append(children, workCommands...)
	children = append(children, versionCommand)
	command.AddCommand(children...)
	return command
}

func newVersionCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:               "version",
		Short:             "Print the CLI version",
		Long:              "Print the activecollab CLI version without contacting the server.",
		Example:           "  activecollab version\n  activecollab version --json",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(_ *cobra.Command, _ []string) error {
			return writeOutput(options.json, map[string]any{"version": options.version})
		},
	}
}

func newInfoCommand(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show ActiveCollab server information and compatibility",
		Long: `Read the server's application and version information and report whether
that exact version is covered by this CLI's compatibility fixtures.`,
		Example:           "  activecollab info\n  activecollab info --json",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := options.client()
			if err != nil {
				return err
			}
			info, err := client.Info(cmd.Context())
			if err != nil {
				return err
			}
			return writeOutput(options.json, info)
		},
	}
}

func (options *rootOptions) client() (*activecollab.Client, error) {
	credentials, err := options.resolveCredentials()
	if err != nil {
		return nil, err
	}
	return activecollab.NewClient(activecollab.Config{
		BaseURL:    credentials.URL,
		Token:      credentials.Token,
		HTTPClient: options.newHTTPClient(),
		UserAgent:  "activecollab-cli/" + options.version,
	})
}

func (options *rootOptions) newHTTPClient() *http.Client {
	if options.httpClient == nil {
		return &http.Client{Timeout: options.timeout}
	}
	client := *options.httpClient
	if client.Timeout == 0 {
		client.Timeout = options.timeout
	}
	return &client
}

func writeOutput(asJSON bool, value any) error {
	var data []byte
	var err error
	if asJSON {
		data, err = json.Marshal(map[string]any{"data": value})
	} else {
		data, err = json.MarshalIndent(value, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(data))
	return err
}

func printError(asJSON bool, err error) {
	if !asJSON {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	code := "command_error"
	status := 0
	var apiError *activecollab.APIError
	if errors.As(err, &apiError) {
		code = "api_error"
		status = apiError.StatusCode
	}
	payload := map[string]any{
		"error": map[string]any{
			"code":        code,
			"message":     err.Error(),
			"http_status": status,
		},
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, `{"error":{"code":"output_error","message":"unable to encode error"}}`)
		return
	}
	fmt.Fprintln(os.Stderr, string(data))
}

func dryRunOutput(options *rootOptions, operation string, payload any) error {
	return writeOutput(options.json, map[string]any{
		"dry_run":   true,
		"operation": operation,
		"payload":   payload,
	})
}
