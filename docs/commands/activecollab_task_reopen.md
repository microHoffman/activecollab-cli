## activecollab task reopen

Reopen a completed task

### Synopsis

Reopen a completed task. The command verifies that the task belongs to the supplied
project before changing its state. Provide a full task URL, or a numeric task ID
together with --project.

```
activecollab task reopen <task-id-or-url> [flags]
```

### Examples

```
  activecollab task reopen 22 --project 7 --dry-run
  activecollab task reopen https://activecollab.example.com/projects/7/tasks/22
```

### Options

```
      --dry-run       validate and show the operation without changing state
  -h, --help          help for reopen
      --project int   project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks

