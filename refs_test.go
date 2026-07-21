package activecollab

import "testing"

func TestParseTaskRef(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		projectID int
		want      TaskRef
		wantError bool
	}{
		{name: "numeric", value: "22", projectID: 7, want: TaskRef{ProjectID: 7, TaskID: 22}},
		{name: "URL", value: "https://activecollab.example/projects/7/tasks/22", want: TaskRef{ProjectID: 7, TaskID: 22}},
		{name: "nested URL", value: "https://activecollab.example/app/projects/7/tasks/22", want: TaskRef{ProjectID: 7, TaskID: 22}},
		{name: "my work modal URL", value: "https://projekty.tomatom.cz/my-work?modal=Task-17905-173", want: TaskRef{ProjectID: 173, TaskID: 17905}},
		{name: "canonical and matching modal", value: "https://activecollab.example/projects/7/tasks/22?modal=Task-22-7", want: TaskRef{ProjectID: 7, TaskID: 22}},
		{name: "missing project", value: "22", wantError: true},
		{name: "conflicting project", value: "https://activecollab.example/projects/7/tasks/22", projectID: 8, wantError: true},
		{name: "conflicting modal project", value: "https://projekty.tomatom.cz/my-work?modal=Task-17905-173", projectID: 8, wantError: true},
		{name: "conflicting path and modal", value: "https://activecollab.example/projects/7/tasks/22?modal=Task-23-7", wantError: true},
		{name: "malformed modal", value: "https://activecollab.example/my-work?modal=Task-22", wantError: true},
		{name: "multiple modal values", value: "https://activecollab.example/my-work?modal=Task-22-7&modal=Task-23-7", wantError: true},
		{name: "oversized modal ID", value: "https://activecollab.example/my-work?modal=Task-999999999999999999999999999999999-7", wantError: true},
		{name: "invalid URL", value: "https://activecollab.example/tasks/22", wantError: true},
		{name: "zero", value: "0", projectID: 7, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseTaskRef(test.value, test.projectID)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected an error, got %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
