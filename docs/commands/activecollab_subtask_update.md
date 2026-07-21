## activecollab subtask update

Update selected subtask fields

### Synopsis

Update only the subtask fields named by flags after verifying that the
subtask belongs to the supplied task. Clearing flags are mutually exclusive
with their corresponding value flags.

```
activecollab subtask update <task-id-or-url> <subtask-id> [flags]
```

### Examples

```
  activecollab subtask update 22 51 --project 7 --name "Retest fix" --dry-run
  activecollab subtask update https://activecollab.example.com/projects/7/tasks/22 51 \
    --clear-assignee --due-on 2026-08-01
```

### Options

```
      --assignee-id int   new assignee user ID
      --clear-assignee    remove the assignee
      --clear-due-on      remove the due date
      --dry-run           validate and show the operation without changing state
      --due-on string     new due date in YYYY-MM-DD format
  -h, --help              help for update
      --name string       new subtask name
      --project int       project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab subtask](activecollab_subtask.md)	 - Create, read, and update task subtasks

