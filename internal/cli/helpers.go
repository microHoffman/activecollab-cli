package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	activecollab "github.com/microHoffman/activecollab-cli"
	"github.com/spf13/cobra"
)

func newCommandGroup(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:                   use,
		Short:                 short,
		Long:                  long,
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
}

func addDryRunFlag(command *cobra.Command, options *rootOptions) {
	command.Flags().BoolVar(&options.dryRun, "dry-run", false, "validate and show the operation without changing state")
}

func markFlagRequired(command *cobra.Command, name string) {
	if err := command.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("mark %s --%s required: %v", command.CommandPath(), name, err))
	}
}

func markFlagFilename(command *cobra.Command, name string) {
	if err := command.MarkFlagFilename(name); err != nil {
		panic(fmt.Sprintf("mark %s --%s as a filename: %v", command.CommandPath(), name, err))
	}
}

func parsePositiveID(name, value string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return id, nil
}

func readBody(body, bodyFile string, stdin io.Reader) (string, error) {
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("use only one of --body or --body-file")
	}
	if bodyFile == "" {
		return body, nil
	}
	var data []byte
	var err error
	if bodyFile == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(bodyFile)
	}
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(data), nil
}

func validateAttachments(paths []string) error {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("attachment %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("attachment %q is not a regular file", path)
		}
	}
	return nil
}

func validateDueDate(cmd *cobra.Command, flagName, value string) error {
	if !cmd.Flags().Changed(flagName) {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("due date must use YYYY-MM-DD")
	}
	return nil
}

func resolveTaskRef(options *rootOptions, value string, projectID int) (*activecollab.Client, activecollab.TaskRef, error) {
	client, err := options.client()
	if err != nil {
		return nil, activecollab.TaskRef{}, err
	}
	ref, err := client.ResolveTaskRef(value, projectID)
	return client, ref, err
}

func requirePositive(name string, value int) error {
	if value <= 0 {
		return fmt.Errorf("%s must be a positive integer", name)
	}
	return nil
}

func validateChangedPositive(cmd *cobra.Command, flagName, valueName string, value int) error {
	if cmd.Flags().Changed(flagName) && value <= 0 {
		return fmt.Errorf("%s must be a positive integer", valueName)
	}
	return nil
}

func changedString(cmd *cobra.Command, name, value string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	copy := value
	return &copy
}

func changedInt(cmd *cobra.Command, name string, value int) *int {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	copy := value
	return &copy
}

func changedBool(cmd *cobra.Command, name string, value bool) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	copy := value
	return &copy
}

func requireBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("comment body is required; use --body or --body-file")
	}
	return nil
}
