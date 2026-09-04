package activecollab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

const testToken = "sensitive-test-token"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", TestedServerVersion, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client, err := NewClient(Config{
		BaseURL:    server.URL + "/api/v1",
		Token:      testToken,
		HTTPClient: server.Client(),
		UserAgent:  "activecollab-cli/test",
	})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, server
}

func TestGetTaskUsesExactContractAndToleratesUnknownFields(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/7/tasks/22" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Angie-AuthApiToken"); got != testToken {
			t.Errorf("token header = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "activecollab-cli/test" {
			t.Errorf("user agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "task.json"))
	}))
	defer server.Close()

	task, err := client.GetTask(context.Background(), TaskRef{ProjectID: 7, TaskID: 22})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != 22 || task.Name != "Synthetic implementation task" {
		t.Fatalf("unexpected task: %#v", task)
	}
	if len(task.Comments) != 1 || task.Comments[0].ID != 41 {
		t.Fatalf("comments not decoded: %#v", task.Comments)
	}
	if len(task.Comments[0].Attachments) != 1 || task.Comments[0].Attachments[0].ID != 32 {
		t.Fatalf("comment attachments not decoded: %#v", task.Comments[0].Attachments)
	}
	if len(task.Subtasks) != 1 || task.Subtasks[0].ID != 51 {
		t.Fatalf("subtasks not decoded: %#v", task.Subtasks)
	}
	if len(task.Attachments) != 1 || task.Attachments[0].ID != 31 {
		t.Fatalf("attachments not decoded: %#v", task.Attachments)
	}
	comments, err := client.ListComments(context.Background(), TaskRef{ProjectID: 7, TaskID: 22})
	if err != nil || len(comments) != 1 || comments[0].ID != 41 {
		t.Fatalf("comments = %#v, err = %v", comments, err)
	}
	subtasks, err := client.ListSubtasks(context.Background(), TaskRef{ProjectID: 7, TaskID: 22})
	if err != nil || len(subtasks) != 1 || subtasks[0].ID != 51 {
		t.Fatalf("subtasks = %#v, err = %v", subtasks, err)
	}
	attachments, err := client.ListAttachments(context.Background(), TaskRef{ProjectID: 7, TaskID: 22})
	if err != nil || len(attachments) != 2 || attachments[0].ID != 31 || attachments[1].ID != 32 {
		t.Fatalf("attachments = %#v, err = %v", attachments, err)
	}
}

func TestUpdateTaskSendsOnlyChangedFields(t *testing.T) {
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/7/tasks/22" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"name": "Renamed"}
		if !reflect.DeepEqual(payload, want) {
			t.Fatalf("payload = %#v, want %#v", payload, want)
		}
		_, _ = w.Write(fixture(t, "task.json"))
	}))
	defer server.Close()

	name := "Renamed"
	_, err := client.UpdateTask(context.Background(), TaskRef{ProjectID: 7, TaskID: 22}, TaskUpdateInput{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateTaskUploadsAttachmentsBeforeMutation(t *testing.T) {
	directory := t.TempDir()
	one := filepath.Join(directory, "one.txt")
	two := filepath.Join(directory, "two.txt")
	if err := os.WriteFile(one, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	var sequence []string
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/upload-files":
			if r.ContentLength != -1 {
				t.Fatalf("multipart upload Content-Length = %d, want streaming request", r.ContentLength)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("attachment_1"); err != nil {
				t.Errorf("attachment_1 missing: %v", err)
			}
			if _, _, err := r.FormFile("attachment_2"); err != nil {
				t.Errorf("attachment_2 missing: %v", err)
			}
			_, _ = w.Write(fixture(t, "uploads.json"))
		case "/api/v1/projects/7/tasks":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			codes, ok := payload["attach_uploaded_files"].([]any)
			if !ok || len(codes) != 2 || codes[0] != "upload-code-one" || codes[1] != "upload-code-two" {
				t.Fatalf("unexpected upload codes: %#v", payload["attach_uploaded_files"])
			}
			_, _ = w.Write(fixture(t, "task.json"))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := client.CreateTask(context.Background(), 7, TaskCreateInput{Name: "With files", Attachments: []string{one, two}})
	if err != nil {
		t.Fatal(err)
	}
	wantSequence := []string{"/api/v1/upload-files", "/api/v1/projects/7/tasks"}
	if !reflect.DeepEqual(sequence, wantSequence) {
		t.Fatalf("sequence = %#v, want %#v", sequence, wantSequence)
	}
}

func TestUploadFailurePreventsParentMutationAndRedactsToken(t *testing.T) {
	file := filepath.Join(t.TempDir(), "attachment.txt")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"type":"UploadError","message":"failed with `+testToken+`"}`)
	}))
	defer server.Close()

	_, err := client.CreateTask(context.Background(), 7, TaskCreateInput{Name: "Should fail", Attachments: []string{file}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatalf("error leaked token: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("got %d requests, want exactly the failed upload", requests.Load())
	}
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestResolveTaskRefRejectsForeignHost(t *testing.T) {
	client, server := testClient(t, http.NotFoundHandler())
	defer server.Close()
	_, err := client.ResolveTaskRef("https://foreign.example/projects/7/tasks/22", 0)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveTaskRefNormalizesDefaultPort(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://ac.example:443/api/v1", Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := client.ResolveTaskRef("https://ac.example/projects/7/tasks/22", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ref != (TaskRef{ProjectID: 7, TaskID: 22}) {
		t.Fatalf("task ref = %#v", ref)
	}
}

func TestResolveTaskRefAcceptsFrontendModalURL(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://projekty.tomatom.cz/api/v1", Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := client.ResolveTaskRef("https://projekty.tomatom.cz/my-work?modal=Task-17905-173", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ref != (TaskRef{ProjectID: 173, TaskID: 17905}) {
		t.Fatalf("task ref = %#v", ref)
	}
}

func TestGetTaskRejectsMismatchedIdentity(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{name: "task ID", response: `{"single":{"id":999,"project_id":7,"name":"Different task"}}`},
		{name: "project ID", response: `{"single":{"id":22,"project_id":999,"name":"Different project"}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, test.response)
			}))
			defer server.Close()

			_, err := client.GetTask(context.Background(), TaskRef{ProjectID: 7, TaskID: 22})
			if err == nil || !strings.Contains(err.Error(), "does not belong") {
				t.Fatalf("unexpected identity error: %v", err)
			}
		})
	}
}

func TestReadEndpointContracts(t *testing.T) {
	seen := make(map[string]int)
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.RequestURI()
		seen[key]++
		w.Header().Set("Content-Type", "application/json")
		switch key {
		case "GET /api/v1/info":
			_, _ = io.WriteString(w, `{"application":"ActiveCollab","version":"7.4.765","future":true}`)
		case "GET /api/v1/projects?page=1":
			_, _ = w.Write(fixture(t, "projects.json"))
		case "GET /api/v1/projects/7":
			_, _ = io.WriteString(w, `{"single":{"id":7,"name":"Synthetic project"}}`)
		case "GET /api/v1/users?page=1":
			_, _ = io.WriteString(w, `[{"id":9,"display_name":"Developer"}]`)
		case "GET /api/v1/users/9":
			_, _ = io.WriteString(w, `{"single":{"id":9,"display_name":"Developer"}}`)
		case "GET /api/v1/projects/7/task-lists?page=1":
			_, _ = io.WriteString(w, `[{"id":3,"project_id":7,"name":"Development"}]`)
		case "GET /api/v1/projects/7/tasks?page=1":
			_, _ = io.WriteString(w, `{"tasks":[{"id":22,"project_id":7,"name":"Synthetic implementation task"}],"future":true}`)
		case "GET /api/v1/history/task/22?verbose=1":
			_, _ = io.WriteString(w, `[{"timestamp":123,"modifications":{"name":["Old","New","Formatted"]}}]`)
		default:
			t.Fatalf("unexpected request: %s", key)
		}
	}))
	defer server.Close()
	ctx := context.Background()

	info, err := client.Info(ctx)
	if err != nil || !info.Tested || info.Version != TestedServerVersion {
		t.Fatalf("info = %#v, err = %v", info, err)
	}
	projects, err := client.ListProjects(ctx)
	if err != nil || len(projects) != 1 || projects[0].ID != 7 {
		t.Fatalf("projects = %#v, err = %v", projects, err)
	}
	project, err := client.GetProject(ctx, 7)
	if err != nil || project.ID != 7 {
		t.Fatalf("project = %#v, err = %v", project, err)
	}
	users, err := client.ListUsers(ctx)
	if err != nil || len(users) != 1 || users[0].ID != 9 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	user, err := client.GetUser(ctx, 9)
	if err != nil || user.ID != 9 {
		t.Fatalf("user = %#v, err = %v", user, err)
	}
	taskLists, err := client.ListTaskLists(ctx, 7)
	if err != nil || len(taskLists) != 1 || taskLists[0].ID != 3 {
		t.Fatalf("task lists = %#v, err = %v", taskLists, err)
	}
	tasks, err := client.ListTasks(ctx, 7)
	if err != nil || len(tasks) != 1 || tasks[0].ID != 22 {
		t.Fatalf("tasks = %#v, err = %v", tasks, err)
	}
	history, err := client.TaskHistory(ctx, 22, true)
	if err != nil || len(history) != 1 || history[0].Timestamp != 123 {
		t.Fatalf("history = %#v, err = %v", history, err)
	}
	if len(seen) != 8 {
		t.Fatalf("saw %d distinct requests, want 8: %#v", len(seen), seen)
	}
}

func TestListProjectsReadsEveryPage(t *testing.T) {
	var requestedPages []string
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		requestedPages = append(requestedPages, page)
		var firstID, count int
		switch page {
		case "1":
			firstID, count = 1, 100
		case "2":
			firstID, count = 101, 100
		case "3":
			firstID, count = 201, 1
		default:
			t.Fatalf("unexpected project page: %q", page)
		}
		projects := make([]Project, count)
		for index := range projects {
			projects[index] = Project{ID: firstID + index, Name: fmt.Sprintf("Project %d", firstID+index)}
		}
		if err := json.NewEncoder(w).Encode(projects); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 201 || projects[0].ID != 1 || projects[200].ID != 201 {
		t.Fatalf("projects = %#v", projects)
	}
	if got, want := strings.Join(requestedPages, ","), "1,2,3"; got != want {
		t.Fatalf("requested pages = %q, want %q", got, want)
	}
}

func TestCollectionEndpointsReadEveryPage(t *testing.T) {
	tests := []struct {
		name string
		path string
		call func(context.Context, *Client) ([]int, error)
	}{
		{
			name: "users",
			path: "/api/v1/users",
			call: func(ctx context.Context, client *Client) ([]int, error) {
				users, err := client.ListUsers(ctx)
				ids := make([]int, len(users))
				for index, user := range users {
					ids[index] = user.ID
				}
				return ids, err
			},
		},
		{
			name: "task lists",
			path: "/api/v1/projects/7/task-lists",
			call: func(ctx context.Context, client *Client) ([]int, error) {
				taskLists, err := client.ListTaskLists(ctx, 7)
				ids := make([]int, len(taskLists))
				for index, taskList := range taskLists {
					ids[index] = taskList.ID
				}
				return ids, err
			},
		},
		{
			name: "tasks",
			path: "/api/v1/projects/7/tasks",
			call: func(ctx context.Context, client *Client) ([]int, error) {
				tasks, err := client.ListTasks(ctx, 7)
				ids := make([]int, len(tasks))
				for index, task := range tasks {
					ids[index] = task.ID
				}
				return ids, err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requestedPages []string
			client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path {
					t.Fatalf("unexpected request path: %s", r.URL.Path)
				}
				page := r.URL.Query().Get("page")
				requestedPages = append(requestedPages, page)
				var firstID, count int
				switch page {
				case "1":
					firstID, count = 1, 100
				case "2":
					firstID, count = 101, 1
				default:
					t.Fatalf("unexpected page: %q", page)
				}
				resources := make([]map[string]any, count)
				for index := range resources {
					resources[index] = map[string]any{"id": firstID + index, "project_id": 7}
				}
				if err := json.NewEncoder(w).Encode(resources); err != nil {
					t.Fatal(err)
				}
			}))
			defer server.Close()

			ids, err := test.call(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != 101 || ids[0] != 1 || ids[100] != 101 {
				t.Fatalf("resource IDs = %#v", ids)
			}
			if got, want := strings.Join(requestedPages, ","), "1,2"; got != want {
				t.Fatalf("requested pages = %q, want %q", got, want)
			}
		})
	}
}

func TestMutationEndpointContracts(t *testing.T) {
	assigneeID := 9
	dueOn := "2026-08-01"
	newName := "Renamed subtask"
	tests := []struct {
		name        string
		method      string
		path        string
		wantPayload map[string]any
		response    string
		verifyOwner bool
		call        func(context.Context, *Client) error
	}{
		{
			name: "complete task", method: http.MethodPut, path: "/api/v1/complete/task/22", wantPayload: map[string]any{},
			response: string(fixture(t, "task.json")),
			call:     func(ctx context.Context, client *Client) error { _, err := client.CompleteTask(ctx, 22); return err },
		},
		{
			name: "reopen task", method: http.MethodPut, path: "/api/v1/open/task/22", wantPayload: map[string]any{},
			response: string(fixture(t, "task.json")),
			call:     func(ctx context.Context, client *Client) error { _, err := client.ReopenTask(ctx, 22); return err },
		},
		{
			name: "add comment", method: http.MethodPost, path: "/api/v1/comments/task/22", wantPayload: map[string]any{"body": "Implemented"},
			response: `{"single":{"id":41,"parent_type":"Task","parent_id":22,"body":"Implemented"}}`,
			call: func(ctx context.Context, client *Client) error {
				comment, err := client.AddComment(ctx, 22, CommentCreateInput{Body: "Implemented"})
				if err == nil && comment.ID != 41 {
					return fmt.Errorf("comment ID = %d", comment.ID)
				}
				return err
			},
		},
		{
			name: "update comment", method: http.MethodPut, path: "/api/v1/comments/41", wantPayload: map[string]any{"body": "Corrected"},
			response: `{"single":{"id":41,"body":"Corrected"}}`,
			call: func(ctx context.Context, client *Client) error {
				_, err := client.UpdateComment(ctx, 41, CommentUpdateInput{Body: "Corrected"})
				return err
			},
		},
		{
			name: "create subtask", method: http.MethodPost, path: "/api/v1/projects/7/tasks/22/subtasks",
			wantPayload: map[string]any{"body": "Add tests", "assignee_id": float64(9), "due_on": "2026-08-01"},
			response:    `{"single":{"id":51,"task_id":22,"project_id":7,"name":"Add tests"}}`,
			call: func(ctx context.Context, client *Client) error {
				_, err := client.CreateSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, SubtaskCreateInput{Name: "Add tests", AssigneeID: &assigneeID, DueOn: &dueOn})
				return err
			},
		},
		{
			name: "update subtask", method: http.MethodPut, path: "/api/v1/projects/7/tasks/22/subtasks/51",
			wantPayload: map[string]any{"body": "Renamed subtask", "due_on": nil},
			response:    `{"single":{"id":51,"task_id":22,"project_id":7,"name":"Renamed subtask"}}`,
			call: func(ctx context.Context, client *Client) error {
				_, err := client.UpdateSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, 51, SubtaskUpdateInput{Name: &newName, ClearDueOn: true})
				return err
			},
		},
		{
			name: "complete subtask", method: http.MethodPut, path: "/api/v1/complete/subtask/51", wantPayload: map[string]any{},
			response:    `{"single":{"id":51,"task_id":22,"project_id":7,"name":"Add tests","is_completed":true}}`,
			verifyOwner: true,
			call: func(ctx context.Context, client *Client) error {
				_, err := client.CompleteSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, 51)
				return err
			},
		},
		{
			name: "reopen subtask", method: http.MethodPut, path: "/api/v1/open/subtask/51", wantPayload: map[string]any{},
			response:    `{"single":{"id":51,"task_id":22,"project_id":7,"name":"Add tests"}}`,
			verifyOwner: true,
			call: func(ctx context.Context, client *Client) error {
				_, err := client.ReopenSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, 51)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.verifyOwner && r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/7/tasks/22/subtasks/51" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"single":{"id":51,"task_id":22,"project_id":7,"name":"Add tests"}}`)
					return
				}
				if r.Method != test.method || r.URL.Path != test.path {
					t.Fatalf("request = %s %s, want %s %s", r.Method, r.URL.Path, test.method, test.path)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Fatalf("content type = %q", got)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(payload, test.wantPayload) {
					t.Fatalf("payload = %#v, want %#v", payload, test.wantPayload)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, test.response)
			}))
			defer server.Close()
			if err := test.call(context.Background(), client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDownloadAttachmentUsesAPIEndpoint(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path != "/api/v1/attachments/31/download" {
			t.Fatalf("download path = %q", r.URL.Path)
		}
		if got := r.Header.Get("X-Angie-AuthApiToken"); got != testToken {
			t.Fatalf("token header = %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "3")
		_, _ = io.WriteString(w, "abc")
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL + "/api/v1", Token: testToken, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	result, err := client.DownloadAttachment(context.Background(), Attachment{
		ID:          31,
		DownloadURL: server.URL + "/proxy.php?proxy=download_file&i=--DOWNLOAD-TOKEN--",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != "abc" || result.Size != 3 || result.ContentType != "text/plain" {
		t.Fatalf("output = %q, result = %#v", output.String(), result)
	}
	wantPaths := []string{"/api/v1/attachments/31/download"}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("download paths = %#v, want %#v", paths, wantPaths)
	}
}

func TestRedirectPolicyProtectsToken(t *testing.T) {
	t.Run("different origin", func(t *testing.T) {
		var targetRequests atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			targetRequests.Add(1)
		}))
		defer target.Close()
		source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
		}))
		defer source.Close()
		client, err := NewClient(Config{BaseURL: source.URL + "/api/v1", Token: testToken, HTTPClient: source.Client()})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Info(context.Background())
		if err == nil || !strings.Contains(err.Error(), "different origin") {
			t.Fatalf("unexpected redirect error: %v", err)
		}
		if targetRequests.Load() != 0 {
			t.Fatalf("unsafe redirect target received %d requests", targetRequests.Load())
		}
	})

	t.Run("HTTPS downgrade", func(t *testing.T) {
		var targetRequests atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			targetRequests.Add(1)
		}))
		defer target.Close()
		source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
		}))
		defer source.Close()
		client, err := NewClient(Config{BaseURL: source.URL + "/api/v1", Token: testToken, HTTPClient: source.Client()})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Info(context.Background())
		if err == nil || !strings.Contains(err.Error(), "different origin") {
			t.Fatalf("unexpected downgrade error: %v", err)
		}
		if targetRequests.Load() != 0 {
			t.Fatalf("downgrade target received %d requests", targetRequests.Load())
		}
	})

	t.Run("same origin", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/info" {
				http.Redirect(w, r, server.URL+"/api/v1/redirected-info", http.StatusFound)
				return
			}
			if r.URL.Path != "/api/v1/redirected-info" {
				t.Fatalf("redirect path = %q", r.URL.Path)
			}
			if got := r.Header.Get("X-Angie-AuthApiToken"); got != testToken {
				t.Fatalf("token header after safe redirect = %q", got)
			}
			_, _ = io.WriteString(w, `{"application":"ActiveCollab","version":"7.4.765"}`)
		}))
		defer server.Close()
		client, err := NewClient(Config{BaseURL: server.URL + "/api/v1", Token: testToken, HTTPClient: server.Client()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Info(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSubtaskOwnershipMismatchPreventsStateMutation(t *testing.T) {
	var stateMutations atomic.Int32
	client, server := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/projects/7/tasks/22/subtasks/51":
			_, _ = io.WriteString(w, `{"single":{"id":51,"task_id":999,"project_id":7,"name":"Different task"}}`)
		case "/api/v1/complete/subtask/51":
			stateMutations.Add(1)
			_, _ = io.WriteString(w, `{"single":{"id":51,"task_id":999,"project_id":7,"name":"Different task"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := client.CompleteSubtask(context.Background(), TaskRef{ProjectID: 7, TaskID: 22}, 51)
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("unexpected ownership error: %v", err)
	}
	if stateMutations.Load() != 0 {
		t.Fatalf("ownership failure made %d state mutations", stateMutations.Load())
	}
}

func TestInvalidInputsFailBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	client, server := testClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	ctx := context.Background()
	zero := 0
	name := "Name"

	checks := []func() error{
		func() error { _, err := client.GetProject(ctx, 0); return err },
		func() error { _, err := client.GetUser(ctx, -1); return err },
		func() error { _, err := client.ListTasks(ctx, 0); return err },
		func() error { _, err := client.CreateTask(ctx, 7, TaskCreateInput{}); return err },
		func() error {
			_, err := client.CreateTask(ctx, 7, TaskCreateInput{Name: "Task", AssigneeID: &zero})
			return err
		},
		func() error {
			_, err := client.UpdateTask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, TaskUpdateInput{})
			return err
		},
		func() error { _, err := client.AddComment(ctx, 22, CommentCreateInput{}); return err },
		func() error {
			_, err := client.CreateSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, SubtaskCreateInput{})
			return err
		},
		func() error {
			_, err := client.UpdateSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, 51, SubtaskUpdateInput{})
			return err
		},
		func() error {
			_, err := client.UpdateSubtask(ctx, TaskRef{ProjectID: 7, TaskID: 22}, 51, SubtaskUpdateInput{Name: &name, AssigneeID: &zero})
			return err
		},
		func() error { _, err := client.DownloadAttachment(ctx, Attachment{}, io.Discard); return err },
	}
	for index, check := range checks {
		if err := check(); err == nil {
			t.Errorf("validation check %d returned no error", index)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid inputs made %d requests", requests.Load())
	}
}

func TestServerVersionCompatibilityIsExplicit(t *testing.T) {
	if !IsTestedServerVersion("7.4.765") {
		t.Fatal("target self-hosted version should be tested")
	}
	if IsTestedServerVersion("8.0.0") {
		t.Fatal("a future version must not be claimed before its contract fixtures pass")
	}
}
