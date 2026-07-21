## activecollab comment update

Update a comment body

### Synopsis

Replace a comment's body by positive numeric comment ID. Use --body-file for
multiline content.

```
activecollab comment update <comment-id> [flags]
```

### Examples

```
  activecollab comment update 41 --body "Corrected result" --dry-run
  activecollab comment update 41 --body-file corrected-result.md
```

### Options

```
      --body string        new comment body
      --body-file string   read comment body from a file, or - for stdin
      --dry-run            validate and show the operation without changing state
  -h, --help               help for update
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab comment](activecollab_comment.md)	 - Read and write task comments

