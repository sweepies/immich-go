# Installation

immich-go is a single binary with no dependencies. Download it and run.

## Pre-built Binaries (Recommended)

### Download

Visit the [releases page](https://github.com/sweepies/immich-go/releases/latest) and download the appropriate archive:

| Platform | Architecture | File |
|----------|--------------|------|
| Windows | x64 | `immich-go_Windows_x86_64.zip` |
| macOS | Intel | `immich-go_Darwin_x86_64.tar.gz` |
| macOS | Apple Silicon | `immich-go_Darwin_arm64.tar.gz` |
| Linux | x64 | `immich-go_Linux_x86_64.tar.gz` |
| Linux | ARM64 | `immich-go_Linux_arm64.tar.gz` |
| FreeBSD | x64 | `immich-go_Freebsd_x86_64.tar.gz` |

### Extract

::: code-group

```bash [Linux/macOS]
tar -xzf immich-go*.tar.gz
```

```powershell [Windows]
# Use Windows Explorer or:
Expand-Archive immich-go*.zip -DestinationPath .
```

:::

### Install (Optional)

Add to your system PATH for easy access:

::: code-group

```bash [Linux/macOS]
sudo mv immich-go /usr/local/bin/
```

```powershell [Windows]
# Move to a directory in your PATH, or add the current directory to PATH
```

:::

### Verify

```bash
immich-go --version
```

## Immich Compatibility

immich-go supports the Immich v2.7.x and v3.x API generations.

Automated compatibility coverage uses deterministic local HTTP contract fixtures pinned to Immich v2.7.5 and v3.1.0. These fixtures exercise the CLI and API request/response contracts without Docker or a live Immich server; they are not live-server end-to-end tests.

The `--device-uuid` upload option is retained for Immich v2 compatibility. immich-go sends the legacy device fields to v2 servers and omits them from v3 uploads.

## Build from Source

Requires Go 1.26 or higher.

```bash
git clone https://github.com/sweepies/immich-go.git
cd immich-go
go build
```

To install to `$GOPATH/bin`:

```bash
go install
```

## Nix

immich-go is available in nixpkgs:

```bash
# Try without installing
nix-shell -p immich-go

# With flakes
nix run nixpkgs#immich-go -- --help
```

For NixOS, add to your configuration:

```nix
environment.systemPackages = with pkgs; [
  immich-go
];
```

## Termux (Android)

Pre-built ARM64 binaries don't work in Termux. Build from source:

```bash
pkg install git golang
git clone https://github.com/sweepies/immich-go.git
cd immich-go
go build

# Add to PATH
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc
```

## API Permissions

Create an API key in Immich (**Account Settings > API Keys**) with these permissions:

### Required Permissions

| Permission | Purpose |
|------------|---------|
| `asset.read` | Read existing assets |
| `asset.upload` | Upload new assets |
| `asset.update` | Update asset metadata |
| `asset.delete` | Delete assets (for stacking) |
| `asset.download` | Download assets (for archive/migration) |
| `asset.copy` | Copy assets between albums |
| `asset.statistics` | Get upload statistics |
| `album.create` | Create albums |
| `album.read` | Read album information |
| `albumAsset.create` | Add assets to albums |
| `stack.create` | Create photo stacks |
| `tag.asset` | Tag assets |
| `tag.create` | Create new tags |
| `server.about` | Get server information |
| `user.read` | Read user information |

### Admin Permissions (for job control)

To pause Immich jobs during upload (recommended for large imports):

| Permission | Purpose |
|------------|---------|
| `job.create` | Control background jobs |
| `job.read` | Read job status |

## Troubleshooting

### Permission Denied (Linux/macOS)

```bash
chmod +x immich-go
```

### Command Not Found

Either:
- Add the binary to your PATH
- Use the full path: `/path/to/immich-go`
- Run from the directory: `./immich-go`

### SSL/TLS Certificate Errors

If you have certificate issues (self-signed certs, corporate proxies):

```bash
immich-go upload --skip-verify-ssl ...
```

::: warning Security Note
Only use `--skip-verify-ssl` in trusted environments.
:::
