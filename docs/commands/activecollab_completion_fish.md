## activecollab completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	activecollab completion fish | source

To load completions for every new session, execute once:

	activecollab completion fish > ~/.config/fish/completions/activecollab.fish

You will need to start a new shell for this setup to take effect.


```
activecollab completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab completion](activecollab_completion.md)	 - Generate the autocompletion script for the specified shell

