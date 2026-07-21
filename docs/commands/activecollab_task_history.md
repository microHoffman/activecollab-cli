## activecollab task history

Read task change history

### Synopsis

Read a task's modification history after verifying its project membership.
Use --verbose to include ActiveCollab's formatted descriptions.

```
activecollab task history <task-id-or-url> [flags]
```

### Examples

```
  activecollab task history 22 --project 7
  activecollab task history https://activecollab.example.com/projects/7/tasks/22 --verbose --json
```

### Options

```
  -h, --help          help for history
      --project int   project ID when using a numeric task ID
      --verbose       include ActiveCollab's formatted modification descriptions
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks

