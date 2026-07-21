## activecollab auth logout

Remove locally stored ActiveCollab credentials

### Synopsis

Remove the token from the OS credential store and delete the local
configuration. This does not revoke the token on the ActiveCollab server and
cannot clear ACTIVECOLLAB_URL or ACTIVECOLLAB_TOKEN in the parent shell.

```
activecollab auth logout [flags]
```

### Examples

```
  activecollab auth logout
  activecollab auth logout --json
```

### Options

```
  -h, --help   help for logout
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab auth](activecollab_auth.md)	 - Authenticate with ActiveCollab

