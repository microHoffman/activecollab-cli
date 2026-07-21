## activecollab auth login

Log in to a self-hosted ActiveCollab server

### Synopsis

Issue and securely store an API token for a self-hosted ActiveCollab server.

Pass the server's complete /api/v1 URL. By default, login prompts for an email
address and a password without echoing the password. Use --token-stdin to save
an existing token supplied by a secret manager. Tokens are stored in the OS
credential store; only the server URL and account name are written to the
ActiveCollab configuration file.

```
activecollab auth login [flags]
```

### Examples

```
  activecollab auth login --url https://activecollab.example.com/api/v1
  secret-manager-command | activecollab auth login --url https://activecollab.example.com/api/v1 --token-stdin
```

### Options

```
      --allow-insecure-http   allow credentials over unencrypted HTTP
      --email string          ActiveCollab account email (prompted when omitted)
  -h, --help                  help for login
      --token-stdin           read an existing API token from standard input
      --url string            complete self-hosted ActiveCollab /api/v1 URL
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab auth](activecollab_auth.md)	 - Authenticate with ActiveCollab

