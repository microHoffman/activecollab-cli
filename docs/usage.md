# Usage guide

This guide covers the common ActiveCollab workflow. The
[generated command reference](commands/activecollab.md) contains every command,
argument, and flag.

## Configure and verify the server

Set the complete API-v1 base URL and an API token in the environment:

```bash
export ACTIVECOLLAB_URL="https://activecollab.example.com/api/v1"
export ACTIVECOLLAB_TOKEN="..."
```

Keep the token in a secret manager or protected shell environment. Do not pass
it as a command-line argument or write it into repository files.

Verify connectivity and exact-version compatibility before other operations:

```bash
activecollab info
activecollab info --json
```

The first release is tested against ActiveCollab Self-Hosted 7.4.765. An
unrecognized version is reported rather than silently claimed as compatible.

## Find projects and tasks

Start with discovery commands when IDs are not known:

```bash
activecollab project list
activecollab task-list list --project 7
activecollab task list --project 7
```

Commands that identify a task accept either:

- a full task URL, which carries both the project and task IDs; or
- a numeric task ID together with `--project`.

```bash
activecollab task get \
  https://activecollab.example.com/projects/7/tasks/22

activecollab task get 22 --project 7
```

Full URLs are preferable when they are available. The CLI verifies that their
origin matches `ACTIVECOLLAB_URL` before attaching credentials.

## Read complete task context

`task get` includes comments, subtasks, and attachment metadata. Focused
commands are available when only one part is needed:

```bash
activecollab task get 22 --project 7
activecollab task history 22 --project 7
activecollab comment list 22 --project 7
activecollab subtask list 22 --project 7
activecollab attachment list 22 --project 7
```

Add `--json` to any command when consuming its stable envelope from a script.

## Preview every state change

Commands that change ActiveCollab or write a downloaded file expose a local
`--dry-run` flag. A dry run validates the target and payload but makes no HTTP
request and does not change local files.

```bash
activecollab task update 22 \
  --project 7 \
  --name "Updated name" \
  --dry-run \
  --json
```

Inspect the preview, then repeat the command without `--dry-run` when the change
is intended:

```bash
activecollab task update 22 --project 7 --name "Updated name"
```

The same pattern applies to task creation, comments, subtasks, completion,
reopening, and attachment downloads.

## Use multiline bodies and attachments safely

Use `--body-file` instead of shell-escaped inline strings for multiline text.
Pass `-` to read from standard input:

```bash
activecollab comment add 22 --project 7 --body-file result.md --dry-run

printf '%s\n' 'Implemented and verified with go test ./...' |
  activecollab comment add 22 --project 7 --body-file - --dry-run
```

`--body` and `--body-file` are mutually exclusive. Repeat `--attach` once per
path; commas in filenames are preserved:

```bash
activecollab task create \
  --project 7 \
  --name "Add contract tests" \
  --body-file task-description.md \
  --attach specification.txt \
  --attach 'trace,part-2.txt' \
  --dry-run
```

Uploads complete before the task or comment is created. If an upload fails, the
parent mutation is not sent.

## Download attachments without clobbering files

List attachments first, then download an ID owned by that task or one of its
comments:

```bash
activecollab attachment list 22 --project 7
activecollab attachment download 22 31 \
  --project 7 \
  --output ./specification.txt \
  --dry-run
```

Downloads are committed only after the complete file is written and synced.
An existing output is preserved unless `--force` is explicitly supplied.

## Get help

Every level of the command tree provides contextual help and examples:

```bash
activecollab --help
activecollab task --help
activecollab task update --help
```

Unknown commands fail with a nonzero exit status. Shell completion can be
enabled by following the [completion guide](completion.md).
