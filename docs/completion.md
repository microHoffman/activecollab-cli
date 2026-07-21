# Shell completion

The CLI uses Cobra's generated completion support for Bash, Zsh, Fish, and
PowerShell. Completion generation is local: it does not contact ActiveCollab or
read the API token.

Inspect the instructions for a shell with:

```bash
activecollab completion bash --help
activecollab completion zsh --help
activecollab completion fish --help
activecollab completion powershell --help
```

Load completion for the current session:

## Bash

```bash
source <(activecollab completion bash)
```

## Zsh

```zsh
source <(activecollab completion zsh)
```

## Fish

```fish
activecollab completion fish | source
```

## PowerShell

```powershell
activecollab completion powershell | Out-String | Invoke-Expression
```

The shell-specific `--help` output describes persistent installation locations.
Regenerate the installed completion script after upgrading the CLI so it stays
synchronized with the available commands and flags.
