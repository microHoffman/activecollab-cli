## activecollab completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(activecollab completion bash)

To load completions for every new session, execute once:

#### Linux:

	activecollab completion bash > /etc/bash_completion.d/activecollab

#### macOS:

	activecollab completion bash > $(brew --prefix)/etc/bash_completion.d/activecollab

You will need to start a new shell for this setup to take effect.


```
activecollab completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --json               emit stable JSON output
      --timeout duration   HTTP request timeout (default 30s)
```

### SEE ALSO

* [activecollab completion](activecollab_completion.md)	 - Generate the autocompletion script for the specified shell

