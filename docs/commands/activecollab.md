## activecollab

Work with ActiveCollab tasks from the command line

### Synopsis

activecollab is an unofficial command-line client for ActiveCollab tasks.

For a self-hosted server, run activecollab auth login and pass its complete
/api/v1 URL. Environment variables remain available for automation. Task
commands accept either a full task URL or a numeric task ID with --project.
Commands that change state provide --dry-run to validate and display the
intended operation without sending it.

### Examples

```
  activecollab auth login --url https://activecollab.example.com/api/v1
  activecollab info
  activecollab task get https://activecollab.example.com/projects/7/tasks/22
  activecollab task update 22 --project 7 --name "Updated name" --dry-run
  activecollab task list --project 7 --json
```

### Options

```
  -h, --help               help for activecollab
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab attachment](activecollab_attachment.md)	 - List and download task and comment attachments
* [activecollab auth](activecollab_auth.md)	 - Authenticate with ActiveCollab
* [activecollab comment](activecollab_comment.md)	 - Read and write task comments
* [activecollab completion](activecollab_completion.md)	 - Generate the autocompletion script for the specified shell
* [activecollab info](activecollab_info.md)	 - Show ActiveCollab server information and compatibility
* [activecollab project](activecollab_project.md)	 - Read ActiveCollab projects
* [activecollab subtask](activecollab_subtask.md)	 - Create, read, and update task subtasks
* [activecollab task](activecollab_task.md)	 - Create, read, and update ActiveCollab tasks
* [activecollab task-list](activecollab_task-list.md)	 - Read ActiveCollab task lists
* [activecollab user](activecollab_user.md)	 - Read ActiveCollab users
* [activecollab version](activecollab_version.md)	 - Print the CLI version

