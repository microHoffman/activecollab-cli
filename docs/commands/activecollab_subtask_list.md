## activecollab subtask list

List a task's subtasks

### Synopsis

List subtasks belonging to a task. Provide a full task URL, or a numeric
task ID together with --project.

```
activecollab subtask list <task-id-or-url> [flags]
```

### Examples

```
  activecollab subtask list 22 --project 7
  activecollab subtask list https://activecollab.example.com/projects/7/tasks/22 --json
```

### Options

```
  -h, --help          help for list
      --project int   project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab subtask](activecollab_subtask.md)	 - Create, read, and update task subtasks

