## activecollab task create

Create a task

### Synopsis

Create a task in one project. Use --body-file for multiline content and
repeat --attach for each file. Files are uploaded before the task is created;
the task is not created if an upload fails.

```
activecollab task create [flags]
```

### Examples

```
  activecollab task create --project 7 --name "Add contract tests" --dry-run
  activecollab task create --project 7 --name "Add contract tests" \
    --body-file task-description.md --assignee-id 9 --attach specification.txt
```

### Options

```
      --assignee-id int      assignee user ID
      --attach stringArray   attachment path (repeatable)
      --body string          task body
      --body-file string     read task body from a file, or - for stdin
      --dry-run              validate and show the operation without changing state
      --due-on string        due date in YYYY-MM-DD format
  -h, --help                 help for create
      --important            mark the task as important
      --name string          task name (required)
      --project int          project ID (required)
      --task-list-id int     task list ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks

