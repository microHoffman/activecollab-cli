## activecollab task update

Update selected task fields

### Synopsis

Update only the task fields named by flags. Clearing flags are mutually
exclusive with their corresponding value flags. Provide a full task URL, or a
numeric task ID together with --project.

```
activecollab task update <task-id-or-url> [flags]
```

### Examples

```
  activecollab task update 22 --project 7 --name "Updated name" --dry-run
  activecollab task update https://activecollab.example.com/projects/7/tasks/22 \
    --body-file updated-description.md --clear-assignee
```

### Options

```
      --assignee-id int      new assignee user ID
      --attach stringArray   attachment path (repeatable)
      --body string          new task body
      --body-file string     read task body from a file, or - for stdin
      --clear-assignee       remove the assignee
      --clear-due-on         remove the due date
      --clear-task-list      remove the task from its task list
      --dry-run              validate and show the operation without changing state
      --due-on string        new due date in YYYY-MM-DD format
  -h, --help                 help for update
      --important            set important status; use --important=false to clear
      --name string          new task name
      --project int          project ID when using a numeric task ID
      --task-list-id int     new task list ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks

