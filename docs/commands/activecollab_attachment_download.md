## activecollab attachment download

Download an owned attachment safely

### Synopsis

Download an attachment only after verifying that it belongs to the supplied
task or one of its comments. The completed download is published atomically.
Existing files are preserved unless --force is explicitly provided.

```
activecollab attachment download <task-id-or-url> <attachment-id> [flags]
```

### Examples

```
  activecollab attachment download 22 31 --project 7 --output ./spec.txt --dry-run
  activecollab attachment download https://activecollab.example.com/projects/7/tasks/22 31 \
    --output ./spec.txt
```

### Options

```
      --dry-run         validate and show the operation without changing state
      --force           replace an existing output file
  -h, --help            help for download
      --output string   output file path (required)
      --project int     project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab attachment](activecollab_attachment.md)	 - List and download task and comment attachments

