# activecollab-cli

Unofficial command-line client for ActiveCollab tasks and coding workflows.
It is designed for both humans and automation, with noninteractive commands and
a stable JSON output mode.

The first release targets **ActiveCollab Self-Hosted 7.4.765** and its
`/api/v1` API. Other server versions are not yet tested or claimed as
compatible.

## Agent skill

An AI-agent workflow skill for this CLI lives in the
[microHoffman/agent-skills](https://github.com/microHoffman/agent-skills)
repository. The skill reads task context before coding and requires explicit
authorization before it posts comments, updates fields, or completes tasks.

Install it independently of the CLI:

```bash
npx skills add https://github.com/microHoffman/agent-skills \
  --skill activecollab \
  --agent '*' \
  --global \
  --yes
```

## Capabilities

- list projects, users, task lists, and project tasks
- read task details, comments, subtasks, history, and task/comment attachments
- create and update tasks and subtasks
- add and update comments
- complete and reopen tasks and subtasks
- upload and safely download task/comment attachments
- preview mutations with `--dry-run`
- emit machine-readable output with `--json`
- issue and store self-hosted API tokens through the OS credential store

Deletion, arbitrary raw API requests, invoicing, and time tracking are outside
the initial scope.

## Installation

With [mise](https://mise.jdx.dev/dev-tools/backends/github.html):

```bash
mise use --global github:microHoffman/activecollab-cli@0.2.0
activecollab version
```

Without mise, download and verify the archive for your OS and architecture from
[GitHub Releases](https://github.com/microHoffman/activecollab-cli/releases).
Exact Linux, macOS, Windows, and source installation instructions are in
[the installation guide](https://github.com/microHoffman/activecollab-cli/blob/main/docs/installation.md).

Go users can install from source:

```bash
go install github.com/microHoffman/activecollab-cli/cmd/activecollab@v0.2.0
```

## Documentation

- [Usage guide](docs/usage.md) for task references, safe reads and writes,
  multiline input, and attachments
- [Command reference](docs/commands/activecollab.md), generated from the Cobra
  command tree used by the binary
- [Shell completion](docs/completion.md) for Bash, Zsh, Fish, and PowerShell
- [Installation guide](docs/installation.md) for all supported platforms

## Configuration

For a self-hosted installation, pass the complete API-v1 URL and log in:

```bash
activecollab auth login \
  --url https://activecollab.example.com/api/v1
activecollab info
```

The login command prompts for the account email and a hidden password, requests
an API token from the self-hosted server, and saves that token in the operating
system credential store. Only the non-secret server URL and account email are
written to the CLI configuration file. HTTPS is required unless
`--allow-insecure-http` is explicitly passed. This follows ActiveCollab's
[self-hosted authentication flow](https://activecollab.com/help/books/self-hosted/self-hosted-api-authentication).

If ActiveCollab already issued a token, pipe it from a secret manager instead
of putting it in shell history:

```bash
secret-manager-command | activecollab auth login \
  --url https://activecollab.example.com/api/v1 \
  --token-stdin
```

Use `activecollab auth status` to inspect the active source without exposing
the token. `activecollab auth logout` removes the local credential but does not
revoke it on the server.

For CI, headless hosts without an OS credential store, or ephemeral sessions,
environment variables remain supported and override saved credentials:

```bash
export ACTIVECOLLAB_URL="https://activecollab.example.com/api/v1"
export ACTIVECOLLAB_TOKEN="..."
```

Never pass a password or token as a command-line argument, commit it, or paste
it into an agent conversation.

## Examples

```bash
# Discover projects and tasks
activecollab project list
activecollab task list --project 7

# A pasted task URL supplies both IDs
activecollab task get \
  https://activecollab.example.com/projects/7/tasks/22

# A numeric task ID requires its project
activecollab task get 22 --project 7 --json

# Create a task with a multiline body and attachment
activecollab task create \
  --project 7 \
  --name "Add contract tests" \
  --body-file task-description.md \
  --assignee-id 9 \
  --attach specification.txt

# Preview a write without making any HTTP request
activecollab task update 22 \
  --project 7 \
  --name "Updated name" \
  --dry-run \
  --json

# Add a comment from stdin
printf '%s\n' 'Implemented and verified with go test ./...' |
  activecollab comment add 22 --project 7 --body-file -

# Complete only when explicitly intended
activecollab task complete 22 --project 7
```

Run `activecollab <resource> <command> --help` for all flags.

## Output

Human-readable formatted JSON is the default. `--json` provides stable
envelopes for automation:

```json
{"data":{"id":22,"project_id":7,"name":"Example"}}
```

Failures are written to stderr and use this shape in JSON mode:

```json
{"error":{"code":"api_error","message":"...","http_status":404}}
```

## Compatibility design

ActiveCollab's [official API documentation](https://developers.activecollab.com/api-documentation/v1/)
describes the `/api/v1` contract used by the target self-hosted release. The
client therefore implements one API-v1 contract instead of guessing at
product-version adapters. Wire responses are mapped to normalized CLI types and
tolerate additive JSON fields.

When another server version is evaluated, the same contract suite will run
against sanitized fixtures from that version. Version-specific endpoint mapping
will be added only when an actual incompatibility is demonstrated.

That is also the path for future v8 support: add an exact-version fixture set
and register the version only after the shared API-v1 contract passes. Introduce
a v8-specific mapper only if verified responses or endpoints actually differ.

## Development and tests

The repository pins Go 1.26.5 for mise users and declares Go 1.25 as the minimum:

```bash
mise install
mise run check
```

Or with an existing supported Go installation:

```bash
go vet ./...
go test -race ./...
```

Automated tests use only in-process fake HTTP servers and synthetic fixtures.
They never connect to or modify a real ActiveCollab workspace. Before a release,
the only manual server check is read-only: `info`, `project list`, and `task get`
for an explicitly selected task.

Command reference pages are generated from the runtime Cobra command tree. When
command metadata or flags change, regenerate and verify them with:

```bash
mise run docs
mise run docs-check
```

## License

MIT. ActiveCollab is a trademark of its respective owner; this project is not
officially affiliated with or endorsed by ActiveCollab.
