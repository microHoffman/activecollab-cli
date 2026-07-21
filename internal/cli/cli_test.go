package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func executeForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = oldStdout })
	type readResult struct {
		data []byte
		err  error
	}
	readDone := make(chan readResult, 1)
	go func() {
		data, err := io.ReadAll(reader)
		readDone <- readResult{data: data, err: err}
	}()
	options := &rootOptions{timeout: 0, version: "test"}
	command := newRootCommand(options)
	command.SetArgs(args)
	executeErr := command.Execute()
	_ = writer.Close()
	os.Stdout = oldStdout
	result := <-readDone
	_ = reader.Close()
	if result.err != nil {
		t.Fatal(result.err)
	}
	return string(result.data), executeErr
}

func configureServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	t.Setenv("ACTIVECOLLAB_URL", server.URL+"/api/v1")
	t.Setenv("ACTIVECOLLAB_TOKEN", "test-token")
}

func TestCommandGroupsRejectUnknownSubcommands(t *testing.T) {
	groups := []string{"project", "user", "task-list", "task", "comment", "subtask", "attachment"}
	for _, group := range groups {
		t.Run(group, func(t *testing.T) {
			_, err := executeForTest(t, group, "not-a-command")
			if err == nil || !strings.Contains(err.Error(), `unknown command "not-a-command"`) {
				t.Fatalf("unexpected command-group error: %v", err)
			}
		})
	}
}

func TestCommandGroupWithoutSubcommandShowsHelp(t *testing.T) {
	output, err := executeForTest(t, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "activecollab task [command]") || !strings.Contains(output, "Available Commands:") {
		t.Fatalf("unexpected task help:\n%s", output)
	}
}

func TestCommandHelpMetadataIsComplete(t *testing.T) {
	root := NewCommand("test")
	root.InitDefaultCompletionCmd()

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command.Hidden {
			return
		}
		if strings.TrimSpace(command.Short) == "" {
			t.Errorf("%s has no short description", command.CommandPath())
		}
		if strings.TrimSpace(command.Long) == "" {
			t.Errorf("%s has no long description", command.CommandPath())
		}
		if !command.HasAvailableSubCommands() &&
			!strings.HasPrefix(command.CommandPath(), "activecollab completion") &&
			strings.TrimSpace(command.Example) == "" {
			t.Errorf("%s has no example", command.CommandPath())
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
}

func TestDryRunFlagIsScopedToStateChangingCommands(t *testing.T) {
	root := NewCommand("test")
	if root.PersistentFlags().Lookup("dry-run") != nil {
		t.Fatal("--dry-run must not be a global flag")
	}

	mutations := [][]string{
		{"task", "create"}, {"task", "update"}, {"task", "complete"}, {"task", "reopen"},
		{"comment", "add"}, {"comment", "update"},
		{"subtask", "create"}, {"subtask", "update"}, {"subtask", "complete"}, {"subtask", "reopen"},
		{"attachment", "download"},
	}
	for _, path := range mutations {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if command.LocalNonPersistentFlags().Lookup("dry-run") == nil {
			t.Errorf("%s has no local --dry-run flag", command.CommandPath())
		}
	}

	reads := [][]string{{"info"}, {"task", "get"}, {"comment", "list"}, {"attachment", "list"}, {"version"}}
	for _, path := range reads {
		command, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if command.LocalNonPersistentFlags().Lookup("dry-run") != nil {
			t.Errorf("%s unexpectedly has --dry-run", command.CommandPath())
		}
	}
}

func TestCobraValidatesRelatedFlagsBeforeExecution(t *testing.T) {
	_, err := executeForTest(
		t,
		"task", "update", "22", "--project", "7", "--assignee-id", "9", "--clear-assignee", "--dry-run",
	)
	if err == nil || !strings.Contains(err.Error(), "none of the others") {
		t.Fatalf("unexpected mutually-exclusive flag error: %v", err)
	}

	_, err = executeForTest(t, "comment", "add", "22", "--project", "7", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "at least one of the flags") {
		t.Fatalf("unexpected one-required flag error: %v", err)
	}
}

func TestCompletionCommandIsAvailable(t *testing.T) {
	output, err := executeForTest(t, "completion", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "__activecollab") {
		t.Fatalf("unexpected bash completion output: %q", output)
	}
}

func TestDryRunMakesNoHTTPRequests(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	configureServer(t, server)

	output, err := executeForTest(t, "--json", "task", "update", "22", "--project", "7", "--name", "Renamed", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatalf("dry run made %d HTTP requests", requests.Load())
	}
	var envelope struct {
		Data struct {
			DryRun    bool   `json:"dry_run"`
			Operation string `json:"operation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output, err)
	}
	if !envelope.Data.DryRun || envelope.Data.Operation != "task.update" {
		t.Fatalf("unexpected dry-run output: %s", output)
	}
}

func TestEveryMutationSupportsNoRequestDryRun(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer server.Close()
	configureServer(t, server)

	tests := []struct {
		operation string
		args      []string
	}{
		{operation: "task.create", args: []string{"task", "create", "--project", "7", "--name", "Task"}},
		{operation: "task.update", args: []string{"task", "update", "22", "--project", "7", "--name", "Renamed"}},
		{operation: "task.complete", args: []string{"task", "complete", "22", "--project", "7"}},
		{operation: "task.reopen", args: []string{"task", "reopen", "22", "--project", "7"}},
		{operation: "comment.add", args: []string{"comment", "add", "22", "--project", "7", "--body", "Done"}},
		{operation: "comment.update", args: []string{"comment", "update", "41", "--body", "Corrected"}},
		{operation: "subtask.create", args: []string{"subtask", "create", "22", "--project", "7", "--name", "Test"}},
		{operation: "subtask.update", args: []string{"subtask", "update", "22", "51", "--project", "7", "--name", "Retest"}},
		{operation: "subtask.complete", args: []string{"subtask", "complete", "22", "51", "--project", "7"}},
		{operation: "subtask.reopen", args: []string{"subtask", "reopen", "22", "51", "--project", "7"}},
		{operation: "attachment.download", args: []string{"attachment", "download", "22", "31", "--project", "7", "--output", "unused.txt"}},
	}
	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			args := append([]string{}, test.args...)
			args = append(args, "--dry-run", "--json")
			output, err := executeForTest(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data struct {
					DryRun    bool            `json:"dry_run"`
					Operation string          `json:"operation"`
					Payload   json.RawMessage `json:"payload"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(output), &envelope); err != nil {
				t.Fatalf("invalid JSON output %q: %v", output, err)
			}
			if !envelope.Data.DryRun || envelope.Data.Operation != test.operation || len(envelope.Data.Payload) == 0 {
				t.Fatalf("unexpected dry-run output: %s", output)
			}
			if strings.Contains(output, `"Name"`) || strings.Contains(output, `"ProjectID"`) {
				t.Fatalf("dry-run output does not use the stable snake-case contract: %s", output)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("dry runs made %d HTTP requests", requests.Load())
	}
}

func TestDryRunRejectsInvalidMutationBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	configureServer(t, server)

	_, err := executeForTest(t, "--json", "task", "update", "22", "--project", "7", "--assignee-id", "0", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "assignee ID must be a positive integer") {
		t.Fatalf("unexpected validation error: %v", err)
	}
	_, err = executeForTest(t, "--json", "task", "update", "22", "--project", "7", "--name", "", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), "task name cannot be empty") {
		t.Fatalf("unexpected empty-name validation error: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid dry run made %d HTTP requests", requests.Load())
	}
}

func TestExplicitEmptyDueDateIsRejected(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	configureServer(t, server)

	tests := []struct {
		name string
		args []string
	}{
		{name: "task create", args: []string{"task", "create", "--project", "7", "--name", "Task", "--due-on", ""}},
		{name: "task update", args: []string{"task", "update", "22", "--project", "7", "--due-on", ""}},
		{name: "subtask create", args: []string{"subtask", "create", "22", "--project", "7", "--name", "Subtask", "--due-on", ""}},
		{name: "subtask update", args: []string{"subtask", "update", "22", "51", "--project", "7", "--due-on", ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{}, test.args...)
			args = append(args, "--dry-run")
			_, err := executeForTest(t, args...)
			if err == nil || !strings.Contains(err.Error(), "due date must use YYYY-MM-DD") {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("empty due-date validation made %d HTTP requests", requests.Load())
	}
}

func TestAttachFlagsPreserveCommaInPath(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	configureServer(t, server)
	attachmentPath := filepath.Join(t.TempDir(), "trace,part-2.txt")
	if err := os.WriteFile(attachmentPath, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "task create", args: []string{"task", "create", "--project", "7", "--name", "Task", "--attach", attachmentPath}},
		{name: "task update", args: []string{"task", "update", "22", "--project", "7", "--attach", attachmentPath}},
		{name: "comment add", args: []string{"comment", "add", "22", "--project", "7", "--body", "Log attached", "--attach", attachmentPath}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{}, test.args...)
			args = append(args, "--dry-run", "--json")
			output, err := executeForTest(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data struct {
					Payload struct {
						Input struct {
							Attachments []string `json:"attachments"`
						} `json:"input"`
					} `json:"payload"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(output), &envelope); err != nil {
				t.Fatalf("invalid JSON output %q: %v", output, err)
			}
			attachments := envelope.Data.Payload.Input.Attachments
			if len(attachments) != 1 || attachments[0] != attachmentPath {
				t.Fatalf("attachment paths = %#v, want [%q]", attachments, attachmentPath)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("attachment dry runs made %d HTTP requests", requests.Load())
	}
}

func TestSubtaskStateVerifiesScopedOwnershipBeforeMutation(t *testing.T) {
	var stateMutations atomic.Int32
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/api/v1/complete/subtask/51" {
			stateMutations.Add(1)
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	configureServer(t, server)

	_, err := executeForTest(t, "subtask", "complete", "22", "51", "--project", "7")
	if err == nil || !strings.Contains(err.Error(), "verify subtask ownership") {
		t.Fatalf("unexpected ownership error: %v", err)
	}
	wantPaths := []string{"/api/v1/projects/7/tasks/22/subtasks/51"}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
	if stateMutations.Load() != 0 {
		t.Fatalf("ownership failure made %d state mutations", stateMutations.Load())
	}
}

func TestTaskIDOnlyMutationsVerifyProjectMembership(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "complete", args: []string{"task", "complete", "22", "--project", "7"}},
		{name: "reopen", args: []string{"task", "reopen", "22", "--project", "7"}},
		{name: "comment", args: []string{"comment", "add", "22", "--project", "7", "--body", "Done"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				http.NotFound(w, r)
			}))
			defer server.Close()
			configureServer(t, server)

			_, err := executeForTest(t, test.args...)
			if err == nil || !strings.Contains(err.Error(), "verify task project membership") {
				t.Fatalf("unexpected membership error: %v", err)
			}
			wantPaths := []string{"/api/v1/projects/7/tasks/22"}
			if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
				t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
			}
		})
	}
}

func TestTaskIDOnlyMutationsRejectMismatchedScopedTask(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		mutationPath string
	}{
		{name: "complete", args: []string{"task", "complete", "22", "--project", "7"}, mutationPath: "/api/v1/complete/task/22"},
		{name: "reopen", args: []string{"task", "reopen", "22", "--project", "7"}, mutationPath: "/api/v1/open/task/22"},
		{name: "comment", args: []string{"comment", "add", "22", "--project", "7", "--body", "Done"}, mutationPath: "/api/v1/comments/task/22"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stateMutations atomic.Int32
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path == test.mutationPath {
					stateMutations.Add(1)
				}
				_, _ = io.WriteString(w, `{"single":{"id":999,"project_id":7,"name":"Different task"}}`)
			}))
			defer server.Close()
			configureServer(t, server)

			_, err := executeForTest(t, test.args...)
			if err == nil || !strings.Contains(err.Error(), "verify task project membership") {
				t.Fatalf("unexpected membership error: %v", err)
			}
			wantPaths := []string{"/api/v1/projects/7/tasks/22"}
			if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
				t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
			}
			if stateMutations.Load() != 0 {
				t.Fatalf("identity mismatch made %d state mutations", stateMutations.Load())
			}
		})
	}
}

func TestTaskHistoryVerifiesProjectMembershipBeforeGlobalLookup(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	configureServer(t, server)

	_, err := executeForTest(t, "task", "history", "22", "--project", "7")
	if err == nil || !strings.Contains(err.Error(), "verify task project membership") {
		t.Fatalf("unexpected membership error: %v", err)
	}
	wantPaths := []string{"/api/v1/projects/7/tasks/22"}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
}

func TestAttachmentDownloadChecksOwnershipAndWritesAtomically(t *testing.T) {
	var paths []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/projects/7/tasks/22":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"single":{"id":22,"project_id":7,"name":"Task","attachments":[{"id":31,"name":"spec.txt","size":3,"download_url":"`+server.URL+`/attachments/31/download"}]},"comments":[],"subtasks":[]}`)
		case "/attachments/31/download":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "abc")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	configureServer(t, server)
	outputPath := filepath.Join(t.TempDir(), "spec.txt")

	output, err := executeForTest(t, "--json", "attachment", "download", "22", "31", "--project", "7", "--output", outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(outputPath); err != nil || string(got) != "abc" {
		t.Fatalf("downloaded file = %q, err = %v", got, err)
	}
	wantPaths := []string{"/api/v1/projects/7/tasks/22", "/attachments/31/download"}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
	if !strings.Contains(output, `"attachment_id":31`) {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestAttachmentDownloadDoesNotOverwriteWithoutForce(t *testing.T) {
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/attachments/31/download" {
			downloads.Add(1)
		}
		_, _ = io.WriteString(w, `{"single":{"id":22,"project_id":7,"name":"Task","attachments":[{"id":31,"name":"spec.txt"}]}}`)
	}))
	defer server.Close()
	configureServer(t, server)
	outputPath := filepath.Join(t.TempDir(), "spec.txt")
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := executeForTest(t, "attachment", "download", "22", "31", "--project", "7", "--output", outputPath)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloads.Load() != 0 {
		t.Fatal("download endpoint was called despite overwrite refusal")
	}
	data, _ := os.ReadFile(outputPath)
	if string(data) != "keep" {
		t.Fatalf("existing file was changed: %q", data)
	}
}

func TestCommitDownloadedFileDoesNotReplaceExistingDestination(t *testing.T) {
	directory := t.TempDir()
	temporaryName := filepath.Join(directory, ".activecollab-download-race")
	outputPath := filepath.Join(directory, "spec.txt")
	if err := os.WriteFile(temporaryName, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := commitDownloadedFile(temporaryName, outputPath, false)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected commit error: %v", err)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("existing file = %q, err = %v", data, readErr)
	}
}

func TestCommitDownloadedFileFallsBackWhenHardLinksAreUnsupported(t *testing.T) {
	directory := t.TempDir()
	temporaryName := filepath.Join(directory, ".activecollab-download-complete")
	outputPath := filepath.Join(directory, "spec.txt")
	if err := os.WriteFile(temporaryName, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupportedLink := func(string, string) error {
		return errors.New("hard links are not supported")
	}

	if err := commitDownloadedFileWithLink(temporaryName, outputPath, false, unsupportedLink); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil || string(data) != "new" {
		t.Fatalf("downloaded file = %q, err = %v", data, err)
	}
	if _, err := os.Stat(temporaryName); !os.IsNotExist(err) {
		t.Fatalf("temporary file still exists or returned unexpected error: %v", err)
	}
}

func TestHardLinkFallbackDoesNotReplaceRacingDestination(t *testing.T) {
	directory := t.TempDir()
	temporaryName := filepath.Join(directory, ".activecollab-download-race")
	outputPath := filepath.Join(directory, "spec.txt")
	if err := os.WriteFile(temporaryName, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsupportedLink := func(string, string) error {
		if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		return errors.New("hard links are not supported")
	}

	err := commitDownloadedFileWithLink(temporaryName, outputPath, false, unsupportedLink)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected commit error: %v", err)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("existing file = %q, err = %v", data, readErr)
	}
}

func TestForcedCommitReplacesExistingDestination(t *testing.T) {
	directory := t.TempDir()
	temporaryName := filepath.Join(directory, ".activecollab-download-complete")
	outputPath := filepath.Join(directory, "spec.txt")
	if err := os.WriteFile(temporaryName, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := commitDownloadedFile(temporaryName, outputPath, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement file = %q, err = %v", data, err)
	}
	if _, err := os.Stat(temporaryName); !os.IsNotExist(err) {
		t.Fatalf("temporary file still exists or returned unexpected error: %v", err)
	}
}

func TestFailedForcedCommitPreservesExistingDestination(t *testing.T) {
	directory := t.TempDir()
	temporaryName := filepath.Join(directory, ".activecollab-download-missing")
	outputPath := filepath.Join(directory, "spec.txt")
	if err := os.WriteFile(outputPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := commitDownloadedFile(temporaryName, outputPath, true)
	if err == nil || !strings.Contains(err.Error(), "atomically replace") {
		t.Fatalf("unexpected replacement error: %v", err)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("existing file = %q, err = %v", data, readErr)
	}
}
