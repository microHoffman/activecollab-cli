package activecollab

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type TaskRef struct {
	ProjectID int `json:"project_id"`
	TaskID    int `json:"task_id"`
}

func ParseTaskRef(value string, projectID int) (TaskRef, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return TaskRef{}, fmt.Errorf("task reference is required")
	}
	if taskID, err := strconv.Atoi(value); err == nil {
		if taskID <= 0 {
			return TaskRef{}, fmt.Errorf("task ID must be positive")
		}
		if projectID <= 0 {
			return TaskRef{}, fmt.Errorf("--project is required when the task reference is a numeric ID")
		}
		return TaskRef{ProjectID: projectID, TaskID: taskID}, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return TaskRef{}, fmt.Errorf("task reference must be a numeric ID or an absolute ActiveCollab task URL")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index := 0; index+3 < len(segments); index++ {
		if segments[index] != "projects" || segments[index+2] != "tasks" {
			continue
		}
		parsedProjectID, projectErr := strconv.Atoi(segments[index+1])
		parsedTaskID, taskErr := strconv.Atoi(segments[index+3])
		if projectErr == nil && taskErr == nil && parsedProjectID > 0 && parsedTaskID > 0 {
			if projectID > 0 && projectID != parsedProjectID {
				return TaskRef{}, fmt.Errorf("--project %d does not match task URL project %d", projectID, parsedProjectID)
			}
			return TaskRef{ProjectID: parsedProjectID, TaskID: parsedTaskID}, nil
		}
	}
	return TaskRef{}, fmt.Errorf("URL does not contain /projects/{project_id}/tasks/{task_id}")
}
