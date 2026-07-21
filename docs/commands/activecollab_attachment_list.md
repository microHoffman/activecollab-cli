## activecollab attachment list

List task and comment attachments

### Synopsis

List attachment metadata from a task and all of its comments. Provide a full
task URL, or a numeric task ID together with --project.

```
activecollab attachment list <task-id-or-url> [flags]
```

### Examples

```
  activecollab attachment list 22 --project 7
  activecollab attachment list https://activecollab.example.com/projects/7/tasks/22 --json
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

* [activecollab attachment](activecollab_attachment.md)	 - List and download task and comment attachments

