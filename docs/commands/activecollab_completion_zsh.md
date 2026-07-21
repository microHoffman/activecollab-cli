## activecollab completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(activecollab completion zsh)

To load completions for every new session, execute once:

#### Linux:

	activecollab completion zsh > "${fpath[1]}/_activecollab"

#### macOS:

	activecollab completion zsh > $(brew --prefix)/share/zsh/site-functions/_activecollab

You will need to start a new shell for this setup to take effect.


```
activecollab completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab completion](activecollab_completion.md)	 - Generate the autocompletion script for the specified shell

