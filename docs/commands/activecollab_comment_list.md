## activecollab comment list

List comments on a task

### Synopsis

List comments on a task. Provide a full task URL, or a numeric task ID
together with --project.

```
activecollab comment list <task-id-or-url> [flags]
```

### Examples

```
  activecollab comment list 22 --project 7
  activecollab comment list https://activecollab.example.com/projects/7/tasks/22 --json
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

* [activecollab comment](activecollab_comment.md)	 - Read and write task comments

