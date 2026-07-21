## activecollab subtask reopen

Reopen a completed subtask

### Synopsis

Reopen a completed subtask. The command verifies that the subtask belongs to the
supplied task before changing its state.

```
activecollab subtask reopen <task-id-or-url> <subtask-id> [flags]
```

### Examples

```
  activecollab subtask reopen 22 51 --project 7 --dry-run
  activecollab subtask reopen https://activecollab.example.com/projects/7/tasks/22 51
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

* [activecollab subtask](activecollab_subtask.md)	 - Create, read, and update task subtasks

