# Installation

Release binaries are self-contained and do not require Go at runtime.

## mise

Install globally:

```bash
mise use --global github:microHoffman/activecollab-cli@0.2.0
activecollab version
```

Omit `--global` to pin the CLI in a project's `mise.toml`:

```bash
mise use github:microHoffman/activecollab-cli@0.2.0
```

Upgrade by selecting a newer version with `mise use`. Remove it with:

```bash
mise uninstall github:microHoffman/activecollab-cli@0.2.0
```

## Linux without mise

Choose `amd64` for x86-64 or `arm64` for 64-bit ARM:

```bash
version=0.2.0
arch=amd64
archive="activecollab_${version}_linux_${arch}.tar.gz"
base="https://github.com/microHoffman/activecollab-cli/releases/download/v${version}"

curl -fLO "${base}/${archive}"
curl -fLO "${base}/checksums.txt"
grep " ${archive}$" checksums.txt | sha256sum --check
tar -xzf "${archive}"
install -Dm755 activecollab "${HOME}/.local/bin/activecollab"
activecollab version
```

Ensure `~/.local/bin` is on `PATH`. To uninstall:

```bash
rm "${HOME}/.local/bin/activecollab"
```

## macOS without mise

Choose `arm64` for Apple Silicon or `amd64` for Intel:

```bash
version=0.2.0
arch=arm64
archive="activecollab_${version}_darwin_${arch}.tar.gz"
base="https://github.com/microHoffman/activecollab-cli/releases/download/v${version}"

curl -fLO "${base}/${archive}"
curl -fLO "${base}/checksums.txt"
grep " ${archive}$" checksums.txt | shasum -a 256 --check
tar -xzf "${archive}"
mkdir -p "${HOME}/.local/bin"
install -m 755 activecollab "${HOME}/.local/bin/activecollab"
activecollab version
```

Remove `~/.local/bin/activecollab` to uninstall.

## Windows without mise

Choose `amd64` for x86-64 or `arm64` for Windows on ARM. In PowerShell:

```powershell
$Version = "0.2.0"
$Arch = "amd64"
$Archive = "activecollab_${Version}_windows_${Arch}.zip"
$Base = "https://github.com/microHoffman/activecollab-cli/releases/download/v${Version}"

Invoke-WebRequest "${Base}/${Archive}" -OutFile $Archive
Invoke-WebRequest "${Base}/checksums.txt" -OutFile "checksums.txt"

$Expected = ((Select-String " $([regex]::Escape($Archive))$" "checksums.txt").Line -split "\s+")[0]
$Actual = (Get-FileHash $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($Actual -ne $Expected.ToLowerInvariant()) { throw "Checksum mismatch" }

Expand-Archive $Archive -DestinationPath ".\activecollab-release" -Force
New-Item -ItemType Directory -Force "$HOME\bin" | Out-Null
Copy-Item ".\activecollab-release\activecollab.exe" "$HOME\bin\activecollab.exe"
& "$HOME\bin\activecollab.exe" version
```

Add `$HOME\bin` to the user `PATH`. Delete `activecollab.exe` to uninstall.

## Install from source

With Go 1.25 or newer:

```bash
go install github.com/microHoffman/activecollab-cli/cmd/activecollab@v0.2.0
```

Ensure `GOBIN`, or `$(go env GOPATH)/bin`, is on `PATH`.

## Configure ActiveCollab

For a self-hosted installation, pass its complete API-v1 base URL to the login
command:

```bash
activecollab auth login \
  --url https://activecollab.example.com/api/v1
activecollab info
```

The CLI prompts for the account email and a hidden password, obtains a token
from the self-hosted `/issue-token` endpoint, validates it, and stores it in the
platform credential store. See ActiveCollab's
[self-hosted authentication documentation](https://activecollab.com/help/books/self-hosted/self-hosted-api-authentication)
for the server-side flow.

Credential storage uses:

- Linux: Secret Service-compatible keyring
- macOS: Keychain
- Windows: Credential Manager

The CLI configuration file contains only the server URL and account email. If
the platform credential store is unavailable, the command fails before asking
for a password or issuing a token.

To save an existing token, pipe it from a secret manager:

```bash
secret-manager-command | activecollab auth login \
  --url https://activecollab.example.com/api/v1 \
  --token-stdin
```

For CI and headless machines, set credentials in a protected environment. They
override saved login credentials:

```bash
export ACTIVECOLLAB_URL="https://activecollab.example.com/api/v1"
export ACTIVECOLLAB_TOKEN="..."
```

Inspect or remove saved credentials without displaying the token:

```bash
activecollab auth status
activecollab auth logout
```

Logout removes only the local credential; revoke the corresponding API token
in ActiveCollab when server-side invalidation is required. Do not put a real
password or token in repository files, shell history, issue descriptions,
command arguments, or agent conversations.

After installation, see the [usage guide](usage.md), the
[generated command reference](commands/activecollab.md), and the
[shell-completion guide](completion.md).
