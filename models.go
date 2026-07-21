package activecollab

const TestedServerVersion = "7.4.765"

var testedServerVersions = map[string]struct{}{
	TestedServerVersion: {},
}

func IsTestedServerVersion(version string) bool {
	_, tested := testedServerVersions[version]
	return tested
}

type Info struct {
	Application string `json:"application"`
	Version     string `json:"version"`
	Tested      bool   `json:"tested"`
}

type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Body        any    `json:"body,omitempty"`
	IsCompleted bool   `json:"is_completed"`
	IsTrashed   bool   `json:"is_trashed"`
	LeaderID    int    `json:"leader_id,omitempty"`
}

type User struct {
	ID               int    `json:"id"`
	DisplayName      string `json:"display_name"`
	ShortDisplayName string `json:"short_display_name,omitempty"`
	Email            string `json:"email,omitempty"`
	IsArchived       bool   `json:"is_archived"`
	IsTrashed        bool   `json:"is_trashed"`
}

type TaskList struct {
	ID          int    `json:"id"`
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	IsCompleted bool   `json:"is_completed"`
	OpenTasks   int    `json:"open_tasks,omitempty"`
}

type Attachment struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	MIMEType     string `json:"mime_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
	Disposition  string `json:"disposition,omitempty"`
	DownloadURL  string `json:"download_url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

type Comment struct {
	ID            int          `json:"id"`
	ParentType    string       `json:"parent_type,omitempty"`
	ParentID      int          `json:"parent_id,omitempty"`
	Body          string       `json:"body"`
	BodyPlainText string       `json:"body_plain_text,omitempty"`
	CreatedOn     int64        `json:"created_on,omitempty"`
	CreatedByID   int          `json:"created_by_id,omitempty"`
	UpdatedOn     int64        `json:"updated_on,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
}

type Subtask struct {
	ID          int    `json:"id"`
	TaskID      int    `json:"task_id"`
	ProjectID   int    `json:"project_id"`
	Name        string `json:"name"`
	AssigneeID  int    `json:"assignee_id,omitempty"`
	IsCompleted bool   `json:"is_completed"`
	DueOn       any    `json:"due_on,omitempty"`
	CreatedOn   int64  `json:"created_on,omitempty"`
	UpdatedOn   int64  `json:"updated_on,omitempty"`
}

type Task struct {
	ID                int          `json:"id"`
	ProjectID         int          `json:"project_id"`
	TaskNumber        int          `json:"task_number,omitempty"`
	TaskListID        int          `json:"task_list_id,omitempty"`
	Name              string       `json:"name"`
	Body              string       `json:"body,omitempty"`
	BodyPlainText     string       `json:"body_plain_text,omitempty"`
	AssigneeID        int          `json:"assignee_id,omitempty"`
	IsCompleted       bool         `json:"is_completed"`
	IsImportant       bool         `json:"is_important"`
	IsTrashed         bool         `json:"is_trashed"`
	DueOn             any          `json:"due_on,omitempty"`
	CreatedOn         int64        `json:"created_on,omitempty"`
	UpdatedOn         int64        `json:"updated_on,omitempty"`
	CommentsCount     int          `json:"comments_count,omitempty"`
	TotalSubtasks     int          `json:"total_subtasks,omitempty"`
	CompletedSubtasks int          `json:"completed_subtasks,omitempty"`
	OpenSubtasks      int          `json:"open_subtasks,omitempty"`
	Attachments       []Attachment `json:"attachments,omitempty"`
	Comments          []Comment    `json:"comments,omitempty"`
	Subtasks          []Subtask    `json:"subtasks,omitempty"`
	TaskList          *TaskList    `json:"task_list,omitempty"`
}

type HistoryEntry struct {
	Timestamp      int64            `json:"timestamp"`
	CreatedByID    int              `json:"created_by_id,omitempty"`
	CreatedByName  string           `json:"created_by_name,omitempty"`
	CreatedByEmail string           `json:"created_by_email,omitempty"`
	Modifications  map[string][]any `json:"modifications"`
}

type Upload struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	MIMEType     string `json:"mime_type,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
}

type TaskCreateInput struct {
	Name        string   `json:"name"`
	Body        *string  `json:"body,omitempty"`
	AssigneeID  *int     `json:"assignee_id,omitempty"`
	DueOn       *string  `json:"due_on,omitempty"`
	TaskListID  *int     `json:"task_list_id,omitempty"`
	IsImportant *bool    `json:"is_important,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
}

type TaskUpdateInput struct {
	Name          *string  `json:"name,omitempty"`
	Body          *string  `json:"body,omitempty"`
	AssigneeID    *int     `json:"assignee_id,omitempty"`
	ClearAssignee bool     `json:"clear_assignee,omitempty"`
	DueOn         *string  `json:"due_on,omitempty"`
	ClearDueOn    bool     `json:"clear_due_on,omitempty"`
	TaskListID    *int     `json:"task_list_id,omitempty"`
	ClearTaskList bool     `json:"clear_task_list,omitempty"`
	IsImportant   *bool    `json:"is_important,omitempty"`
	Attachments   []string `json:"attachments,omitempty"`
}

type CommentCreateInput struct {
	Body        string   `json:"body"`
	Attachments []string `json:"attachments,omitempty"`
}

type CommentUpdateInput struct {
	Body string `json:"body"`
}

type SubtaskCreateInput struct {
	Name       string  `json:"name"`
	AssigneeID *int    `json:"assignee_id,omitempty"`
	DueOn      *string `json:"due_on,omitempty"`
}

type SubtaskUpdateInput struct {
	Name          *string `json:"name,omitempty"`
	AssigneeID    *int    `json:"assignee_id,omitempty"`
	ClearAssignee bool    `json:"clear_assignee,omitempty"`
	DueOn         *string `json:"due_on,omitempty"`
	ClearDueOn    bool    `json:"clear_due_on,omitempty"`
}
