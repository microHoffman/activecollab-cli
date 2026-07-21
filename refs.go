package activecollab

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var modalTaskPattern = regexp.MustCompile(`^Task-([1-9][0-9]*)-([1-9][0-9]*)$`)

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
	pathRef, pathOK := parsePathTaskRef(parsed.Path)
	modalRef, modalOK, modalErr := parseModalTaskRef(parsed.Query()["modal"])
	if modalErr != nil {
		return TaskRef{}, modalErr
	}
	if pathOK && modalOK && pathRef != modalRef {
		return TaskRef{}, fmt.Errorf("task URL path and modal query identify different tasks")
	}
	ref := pathRef
	if !pathOK {
		ref = modalRef
	}
	if !pathOK && !modalOK {
		return TaskRef{}, fmt.Errorf("URL does not contain /projects/{project_id}/tasks/{task_id} or modal=Task-{task_id}-{project_id}")
	}
	if projectID > 0 && projectID != ref.ProjectID {
		return TaskRef{}, fmt.Errorf("--project %d does not match task URL project %d", projectID, ref.ProjectID)
	}
	return ref, nil
}

func parsePathTaskRef(path string) (TaskRef, bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for index := 0; index+3 < len(segments); index++ {
		if segments[index] != "projects" || segments[index+2] != "tasks" {
			continue
		}
		parsedProjectID, projectErr := strconv.Atoi(segments[index+1])
		parsedTaskID, taskErr := strconv.Atoi(segments[index+3])
		if projectErr == nil && taskErr == nil && parsedProjectID > 0 && parsedTaskID > 0 {
			return TaskRef{ProjectID: parsedProjectID, TaskID: parsedTaskID}, true
		}
	}
	return TaskRef{}, false
}

func parseModalTaskRef(values []string) (TaskRef, bool, error) {
	if len(values) == 0 {
		return TaskRef{}, false, nil
	}
	if len(values) != 1 {
		return TaskRef{}, false, errors.New("task URL contains multiple modal values")
	}
	matches := modalTaskPattern.FindStringSubmatch(values[0])
	if matches == nil {
		return TaskRef{}, false, fmt.Errorf("task URL modal must use Task-{task_id}-{project_id}")
	}
	taskID, taskErr := strconv.Atoi(matches[1])
	projectID, projectErr := strconv.Atoi(matches[2])
	if taskErr != nil || projectErr != nil {
		return TaskRef{}, false, errors.New("task URL modal contains an ID that is too large")
	}
	return TaskRef{ProjectID: projectID, TaskID: taskID}, true, nil
}
