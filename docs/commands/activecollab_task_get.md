## activecollab task get

Read a task with comments, subtasks, and attachments

### Synopsis

Read a task and its embedded comments, subtasks, and attachment metadata.

Provide a full task URL, or provide a numeric task ID together with --project.

```
activecollab task get <task-id-or-url> [flags]
```

### Examples

```
  activecollab task get https://activecollab.example.com/projects/7/tasks/22
  activecollab task get 22 --project 7 --json
```

### Options

```
  -h, --help          help for get
      --project int   project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks

