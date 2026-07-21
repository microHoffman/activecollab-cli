## activecollab subtask create

Create a subtask

### Synopsis

Create a subtask under an existing task. Provide a full task URL, or a
numeric task ID together with --project.

```
activecollab subtask create <task-id-or-url> [flags]
```

### Examples

```
  activecollab subtask create 22 --project 7 --name "Add regression test" --dry-run
  activecollab subtask create https://activecollab.example.com/projects/7/tasks/22 \
    --name "Add regression test" --assignee-id 9 --due-on 2026-08-01
```

### Options

```
      --assignee-id int   assignee user ID
      --dry-run           validate and show the operation without changing state
      --due-on string     due date in YYYY-MM-DD format
  -h, --help              help for create
      --name string       subtask name (required)
      --project int       project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab subtask](activecollab_subtask.md)	 - Create, read, and update task subtasks

