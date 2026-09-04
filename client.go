package activecollab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/microHoffman/activecollab-cli/internal/transport"
)

type Config struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	UserAgent  string
}

type Client struct {
	transport *transport.Client
}

type DownloadResult struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type,omitempty"`
}

func NewClient(config Config) (*Client, error) {
	transportClient, err := transport.New(transport.Config{
		BaseURL:    config.BaseURL,
		Token:      config.Token,
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	})
	if err != nil {
		return nil, err
	}
	return &Client{transport: transportClient}, nil
}

func (c *Client) ResolveTaskRef(value string, projectID int) (TaskRef, error) {
	ref, err := ParseTaskRef(value, projectID)
	if err != nil {
		return TaskRef{}, err
	}
	parsed, parseErr := url.Parse(strings.TrimSpace(value))
	if parseErr == nil && parsed.IsAbs() {
		base := c.transport.BaseURL()
		if !transport.SameOrigin(parsed, base) {
			return TaskRef{}, fmt.Errorf("task URL host %q does not match configured ActiveCollab host %q", parsed.Host, base.Host)
		}
	}
	return ref, nil
}

func (c *Client) Info(ctx context.Context) (Info, error) {
	data, err := c.transport.DoJSON(ctx, http.MethodGet, "/info", nil, nil)
	if err != nil {
		return Info{}, normalizeError(err)
	}
	var info Info
	if err := decodeDirect(data, &info); err != nil {
		return Info{}, err
	}
	if info.Application == "" || info.Version == "" {
		return Info{}, fmt.Errorf("ActiveCollab info response is missing application or version")
	}
	info.Tested = IsTestedServerVersion(info.Version)
	return info, nil
}

func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	return listResource[Project](ctx, c, "/projects", "projects")
}

func (c *Client) GetProject(ctx context.Context, projectID int) (Project, error) {
	if projectID <= 0 {
		return Project{}, fmt.Errorf("project ID must be positive")
	}
	return singleResource[Project](ctx, c, "/projects/"+strconv.Itoa(projectID))
}

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	return listResource[User](ctx, c, "/users", "users")
}

func (c *Client) GetUser(ctx context.Context, userID int) (User, error) {
	if userID <= 0 {
		return User{}, fmt.Errorf("user ID must be positive")
	}
	return singleResource[User](ctx, c, "/users/"+strconv.Itoa(userID))
}

func (c *Client) ListTaskLists(ctx context.Context, projectID int) ([]TaskList, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("project ID must be positive")
	}
	path := fmt.Sprintf("/projects/%d/task-lists", projectID)
	return listResource[TaskList](ctx, c, path, "task_lists")
}

func (c *Client) ListTasks(ctx context.Context, projectID int) ([]Task, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("project ID must be positive")
	}
	path := fmt.Sprintf("/projects/%d/tasks", projectID)
	return listResource[Task](ctx, c, path, "tasks")
}

func (c *Client) GetTask(ctx context.Context, ref TaskRef) (Task, error) {
	if err := validateTaskRef(ref); err != nil {
		return Task{}, err
	}
	path := fmt.Sprintf("/projects/%d/tasks/%d", ref.ProjectID, ref.TaskID)
	data, err := c.transport.DoJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return Task{}, normalizeError(err)
	}
	task, err := decodeTask(data)
	if err != nil {
		return Task{}, err
	}
	if task.ID != ref.TaskID || task.ProjectID != ref.ProjectID {
		return Task{}, fmt.Errorf("task %d does not belong to project %d", ref.TaskID, ref.ProjectID)
	}
	return task, nil
}

func (c *Client) CreateTask(ctx context.Context, projectID int, input TaskCreateInput) (Task, error) {
	if projectID <= 0 {
		return Task{}, fmt.Errorf("project ID must be positive")
	}
	if strings.TrimSpace(input.Name) == "" {
		return Task{}, fmt.Errorf("task name is required")
	}
	if err := validateOptionalPositive("assignee ID", input.AssigneeID); err != nil {
		return Task{}, err
	}
	if err := validateOptionalPositive("task list ID", input.TaskListID); err != nil {
		return Task{}, err
	}
	payload := map[string]any{"name": input.Name}
	setOptionalTaskFields(payload, input.Body, input.AssigneeID, input.DueOn, input.TaskListID, input.IsImportant)
	if err := c.addAttachments(ctx, payload, input.Attachments); err != nil {
		return Task{}, err
	}
	return c.mutateTask(ctx, http.MethodPost, fmt.Sprintf("/projects/%d/tasks", projectID), payload)
}

func (c *Client) UpdateTask(ctx context.Context, ref TaskRef, input TaskUpdateInput) (Task, error) {
	if err := validateTaskRef(ref); err != nil {
		return Task{}, err
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return Task{}, fmt.Errorf("task name cannot be empty")
	}
	if err := validateOptionalPositive("assignee ID", input.AssigneeID); err != nil {
		return Task{}, err
	}
	if err := validateOptionalPositive("task list ID", input.TaskListID); err != nil {
		return Task{}, err
	}
	if input.AssigneeID != nil && input.ClearAssignee {
		return Task{}, fmt.Errorf("cannot set and clear assignee in the same update")
	}
	if input.DueOn != nil && input.ClearDueOn {
		return Task{}, fmt.Errorf("cannot set and clear due date in the same update")
	}
	if input.TaskListID != nil && input.ClearTaskList {
		return Task{}, fmt.Errorf("cannot set and clear task list in the same update")
	}
	payload := map[string]any{}
	setOptionalTaskFields(payload, input.Body, input.AssigneeID, input.DueOn, input.TaskListID, input.IsImportant)
	if input.Name != nil {
		payload["name"] = *input.Name
	}
	if input.ClearAssignee {
		payload["assignee_id"] = 0
	}
	if input.ClearDueOn {
		payload["due_on"] = nil
	}
	if input.ClearTaskList {
		payload["task_list_id"] = 0
	}
	if err := c.addAttachments(ctx, payload, input.Attachments); err != nil {
		return Task{}, err
	}
	if len(payload) == 0 {
		return Task{}, fmt.Errorf("at least one task field must be updated")
	}
	path := fmt.Sprintf("/projects/%d/tasks/%d", ref.ProjectID, ref.TaskID)
	return c.mutateTask(ctx, http.MethodPut, path, payload)
}

func (c *Client) CompleteTask(ctx context.Context, taskID int) (Task, error) {
	if taskID <= 0 {
		return Task{}, fmt.Errorf("task ID must be positive")
	}
	return c.mutateTask(ctx, http.MethodPut, fmt.Sprintf("/complete/task/%d", taskID), nil)
}

func (c *Client) ReopenTask(ctx context.Context, taskID int) (Task, error) {
	if taskID <= 0 {
		return Task{}, fmt.Errorf("task ID must be positive")
	}
	return c.mutateTask(ctx, http.MethodPut, fmt.Sprintf("/open/task/%d", taskID), nil)
}

func (c *Client) TaskHistory(ctx context.Context, taskID int, verbose bool) ([]HistoryEntry, error) {
	if taskID <= 0 {
		return nil, fmt.Errorf("task ID must be positive")
	}
	query := url.Values{}
	if verbose {
		query.Set("verbose", "1")
	}
	data, err := c.transport.DoJSON(ctx, http.MethodGet, fmt.Sprintf("/history/task/%d", taskID), query, nil)
	if err != nil {
		return nil, normalizeError(err)
	}
	var entries []HistoryEntry
	if err := decodeDirect(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) ListComments(ctx context.Context, ref TaskRef) ([]Comment, error) {
	task, err := c.GetTask(ctx, ref)
	return task.Comments, err
}

func (c *Client) AddComment(ctx context.Context, taskID int, input CommentCreateInput) (Comment, error) {
	if taskID <= 0 {
		return Comment{}, fmt.Errorf("task ID must be positive")
	}
	if strings.TrimSpace(input.Body) == "" {
		return Comment{}, fmt.Errorf("comment body is required")
	}
	payload := map[string]any{"body": input.Body}
	if err := c.addAttachments(ctx, payload, input.Attachments); err != nil {
		return Comment{}, err
	}
	return mutateSingle[Comment](ctx, c, http.MethodPost, fmt.Sprintf("/comments/task/%d", taskID), payload)
}

func (c *Client) UpdateComment(ctx context.Context, commentID int, input CommentUpdateInput) (Comment, error) {
	if commentID <= 0 {
		return Comment{}, fmt.Errorf("comment ID must be positive")
	}
	if strings.TrimSpace(input.Body) == "" {
		return Comment{}, fmt.Errorf("comment body is required")
	}
	return mutateSingle[Comment](ctx, c, http.MethodPut, fmt.Sprintf("/comments/%d", commentID), map[string]any{"body": input.Body})
}

func (c *Client) ListSubtasks(ctx context.Context, ref TaskRef) ([]Subtask, error) {
	task, err := c.GetTask(ctx, ref)
	return task.Subtasks, err
}

func (c *Client) CreateSubtask(ctx context.Context, ref TaskRef, input SubtaskCreateInput) (Subtask, error) {
	if err := validateTaskRef(ref); err != nil {
		return Subtask{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return Subtask{}, fmt.Errorf("subtask name is required")
	}
	if err := validateOptionalPositive("assignee ID", input.AssigneeID); err != nil {
		return Subtask{}, err
	}
	payload := map[string]any{"body": input.Name}
	if input.AssigneeID != nil {
		payload["assignee_id"] = *input.AssigneeID
	}
	if input.DueOn != nil {
		payload["due_on"] = *input.DueOn
	}
	path := fmt.Sprintf("/projects/%d/tasks/%d/subtasks", ref.ProjectID, ref.TaskID)
	return mutateSingle[Subtask](ctx, c, http.MethodPost, path, payload)
}

func (c *Client) UpdateSubtask(ctx context.Context, ref TaskRef, subtaskID int, input SubtaskUpdateInput) (Subtask, error) {
	if err := validateTaskRef(ref); err != nil {
		return Subtask{}, err
	}
	if subtaskID <= 0 {
		return Subtask{}, fmt.Errorf("subtask ID must be positive")
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return Subtask{}, fmt.Errorf("subtask name cannot be empty")
	}
	if err := validateOptionalPositive("assignee ID", input.AssigneeID); err != nil {
		return Subtask{}, err
	}
	if input.AssigneeID != nil && input.ClearAssignee {
		return Subtask{}, fmt.Errorf("cannot set and clear assignee in the same update")
	}
	if input.DueOn != nil && input.ClearDueOn {
		return Subtask{}, fmt.Errorf("cannot set and clear due date in the same update")
	}
	payload := map[string]any{}
	if input.Name != nil {
		payload["body"] = *input.Name
	}
	if input.AssigneeID != nil {
		payload["assignee_id"] = *input.AssigneeID
	}
	if input.ClearAssignee {
		payload["assignee_id"] = 0
	}
	if input.DueOn != nil {
		payload["due_on"] = *input.DueOn
	}
	if input.ClearDueOn {
		payload["due_on"] = nil
	}
	if len(payload) == 0 {
		return Subtask{}, fmt.Errorf("at least one subtask field must be updated")
	}
	path := fmt.Sprintf("/projects/%d/tasks/%d/subtasks/%d", ref.ProjectID, ref.TaskID, subtaskID)
	return mutateSingle[Subtask](ctx, c, http.MethodPut, path, payload)
}

func (c *Client) GetSubtask(ctx context.Context, ref TaskRef, subtaskID int) (Subtask, error) {
	if err := validateTaskRef(ref); err != nil {
		return Subtask{}, err
	}
	if subtaskID <= 0 {
		return Subtask{}, fmt.Errorf("subtask ID must be positive")
	}
	path := fmt.Sprintf("/projects/%d/tasks/%d/subtasks/%d", ref.ProjectID, ref.TaskID, subtaskID)
	subtask, err := singleResource[Subtask](ctx, c, path)
	if err != nil {
		return Subtask{}, err
	}
	if subtask.ID != subtaskID || subtask.TaskID != ref.TaskID || subtask.ProjectID != ref.ProjectID {
		return Subtask{}, fmt.Errorf("subtask %d does not belong to task %d in project %d", subtaskID, ref.TaskID, ref.ProjectID)
	}
	return subtask, nil
}

func (c *Client) CompleteSubtask(ctx context.Context, ref TaskRef, subtaskID int) (Subtask, error) {
	if _, err := c.GetSubtask(ctx, ref, subtaskID); err != nil {
		return Subtask{}, fmt.Errorf("verify subtask ownership: %w", err)
	}
	return mutateSingle[Subtask](ctx, c, http.MethodPut, fmt.Sprintf("/complete/subtask/%d", subtaskID), nil)
}

func (c *Client) ReopenSubtask(ctx context.Context, ref TaskRef, subtaskID int) (Subtask, error) {
	if _, err := c.GetSubtask(ctx, ref, subtaskID); err != nil {
		return Subtask{}, fmt.Errorf("verify subtask ownership: %w", err)
	}
	return mutateSingle[Subtask](ctx, c, http.MethodPut, fmt.Sprintf("/open/subtask/%d", subtaskID), nil)
}

func (c *Client) ListAttachments(ctx context.Context, ref TaskRef) ([]Attachment, error) {
	task, err := c.GetTask(ctx, ref)
	if err != nil {
		return nil, err
	}
	attachments := make([]Attachment, 0, len(task.Attachments))
	seen := make(map[int]struct{})
	add := func(attachment Attachment) {
		if attachment.ID <= 0 {
			return
		}
		if _, exists := seen[attachment.ID]; exists {
			return
		}
		seen[attachment.ID] = struct{}{}
		attachments = append(attachments, attachment)
	}
	for _, attachment := range task.Attachments {
		add(attachment)
	}
	for _, comment := range task.Comments {
		for _, attachment := range comment.Attachments {
			add(attachment)
		}
	}
	return attachments, nil
}

func (c *Client) DownloadAttachment(ctx context.Context, attachment Attachment, target io.Writer) (DownloadResult, error) {
	if attachment.ID <= 0 {
		return DownloadResult{}, fmt.Errorf("attachment ID must be positive")
	}
	baseURL := c.transport.BaseURL()
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + fmt.Sprintf("/attachments/%d/download", attachment.ID)
	baseURL.RawPath = ""
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	location := baseURL.String()
	body, contentLength, contentType, err := c.transport.Download(ctx, location)
	if err != nil {
		return DownloadResult{}, normalizeError(err)
	}
	defer body.Close()
	written, err := io.Copy(target, body)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("download attachment: %w", err)
	}
	if contentLength >= 0 && contentLength != written {
		return DownloadResult{}, fmt.Errorf("download attachment: expected %d bytes, received %d", contentLength, written)
	}
	return DownloadResult{Size: written, ContentType: contentType}, nil
}

func (c *Client) addAttachments(ctx context.Context, payload map[string]any, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	data, err := c.transport.UploadFiles(ctx, paths)
	if err != nil {
		return normalizeError(err)
	}
	var uploads []Upload
	if err := decodeDirect(data, &uploads); err != nil {
		return err
	}
	codes := make([]string, 0, len(uploads))
	for _, upload := range uploads {
		if upload.Code == "" {
			return fmt.Errorf("upload response is missing file code")
		}
		codes = append(codes, upload.Code)
	}
	if len(codes) != len(paths) {
		return fmt.Errorf("upload response returned %d files for %d attachments", len(codes), len(paths))
	}
	payload["attach_uploaded_files"] = codes
	return nil
}

func (c *Client) mutateTask(ctx context.Context, method, path string, payload any) (Task, error) {
	data, err := c.transport.DoJSON(ctx, method, path, nil, payload)
	if err != nil {
		return Task{}, normalizeError(err)
	}
	return decodeTask(data)
}

func mutateSingle[T any](ctx context.Context, c *Client, method, path string, payload any) (T, error) {
	var zero T
	data, err := c.transport.DoJSON(ctx, method, path, nil, payload)
	if err != nil {
		return zero, normalizeError(err)
	}
	return decodeSingle[T](data)
}

func listResource[T any](ctx context.Context, c *Client, path, key string) ([]T, error) {
	const pageSize = 100

	resources := make([]T, 0)
	for page := 1; ; page++ {
		query := url.Values{"page": {strconv.Itoa(page)}}
		data, err := c.transport.DoJSON(ctx, http.MethodGet, path, query, nil)
		if err != nil {
			return nil, normalizeError(err)
		}
		pageResources, err := decodeList[T](data, key)
		if err != nil {
			return nil, err
		}
		resources = append(resources, pageResources...)
		if len(pageResources) < pageSize {
			return resources, nil
		}
	}
}

func singleResource[T any](ctx context.Context, c *Client, path string) (T, error) {
	var zero T
	data, err := c.transport.DoJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return zero, normalizeError(err)
	}
	return decodeSingle[T](data)
}

func decodeTask(data []byte) (Task, error) {
	var response struct {
		Single   Task      `json:"single"`
		Comments []Comment `json:"comments"`
		Subtasks []Subtask `json:"subtasks"`
		TaskList *TaskList `json:"task_list"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return Task{}, fmt.Errorf("decode API response: %w", err)
	}
	if response.Single.ID == 0 {
		var direct Task
		if err := json.Unmarshal(data, &direct); err != nil || direct.ID == 0 {
			return Task{}, fmt.Errorf("task response is missing task ID")
		}
		return direct, nil
	}
	response.Single.Comments = response.Comments
	response.Single.Subtasks = response.Subtasks
	response.Single.TaskList = response.TaskList
	return response.Single, nil
}

func decodeSingle[T any](data []byte) (T, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		var zero T
		return zero, fmt.Errorf("decode API response: %w", err)
	}
	if raw, ok := object["single"]; ok {
		var single T
		if err := json.Unmarshal(raw, &single); err != nil {
			var zero T
			return zero, fmt.Errorf("decode API response field %q: %w", "single", err)
		}
		return single, nil
	}
	var direct T
	if err := json.Unmarshal(data, &direct); err != nil {
		var zero T
		return zero, fmt.Errorf("decode API response: %w", err)
	}
	return direct, nil
}

func decodeList[T any](data []byte, key string) ([]T, error) {
	var direct []T
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("decode API response: %w", err)
	}
	raw, ok := object[key]
	if !ok {
		return nil, fmt.Errorf("API response is missing %q", key)
	}
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("decode API response field %q: %w", key, err)
	}
	return direct, nil
}

func decodeDirect(data []byte, target any) error {
	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("ActiveCollab API returned an empty response")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode API response: %w", err)
	}
	return nil
}

func setOptionalTaskFields(payload map[string]any, body *string, assigneeID *int, dueOn *string, taskListID *int, important *bool) {
	if body != nil {
		payload["body"] = *body
	}
	if assigneeID != nil {
		payload["assignee_id"] = *assigneeID
	}
	if dueOn != nil {
		payload["due_on"] = *dueOn
	}
	if taskListID != nil {
		payload["task_list_id"] = *taskListID
	}
	if important != nil {
		payload["is_important"] = *important
	}
}

func validateTaskRef(ref TaskRef) error {
	if ref.ProjectID <= 0 || ref.TaskID <= 0 {
		return fmt.Errorf("project ID and task ID must be positive")
	}
	return nil
}

func validateOptionalPositive(name string, value *int) error {
	if value != nil && *value <= 0 {
		return fmt.Errorf("%s must be positive", name)
	}
	return nil
}
