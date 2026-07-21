## activecollab comment add

Add a comment to a task

### Synopsis

Add a nonempty comment to a task. Use --body-file for multiline content and
repeat --attach for each file. The command verifies task membership before
creating the comment.

```
activecollab comment add <task-id-or-url> [flags]
```

### Examples

```
  activecollab comment add 22 --project 7 --body "Implemented and tested" --dry-run
  activecollab comment add https://activecollab.example.com/projects/7/tasks/22 \
    --body-file result.md --attach test-output.txt
```

### Options

```
      --attach stringArray   attachment path (repeatable)
      --body string          comment body
      --body-file string     read comment body from a file, or - for stdin
      --dry-run              validate and show the operation without changing state
  -h, --help                 help for add
      --project int          project ID when using a numeric task ID
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab comment](activecollab_comment.md)	 - Read and write task comments

