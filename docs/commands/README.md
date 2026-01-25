# Command Reference

Immich-Go uses a simple command structure with global options, commands, source flags, and command options.

## Command Structure

```bash
immich-go [global-options] command [source-flags] [command-options] [path]
```

## Available Commands

| Command | Description | Source Flags |
|---------|-------------|-------------------------------|
| [upload](upload.md) | Upload photos/videos to Immich server | `--google`, `--icloud`, `--picasa`, `--from-immich` |
| [archive](archive.md) | Export/archive photos to local folder structure | `--google`, `--icloud`, `--picasa`, `--from-immich` |
| [stack](stack.md) | Organize related photos into stacks on server | (none) |
| version | Display version information | (none) |

## Global Options

These options work with all commands:

| Option | Default | Description |
|--------|---------|-------------|
| `-h, --help` | - | Show help information |
| `--log-level` | `INFO` | Set logging level: DEBUG, INFO, WARN, ERROR |
| `-v, --version` | - | Display current version |

Logs are written to stderr. Redirect output to capture a file (e.g., `2> logs.txt`).

## Environment Variables

| Variable | Description |
|----------|-------------|
| `IMMICHGO_TEMPDIR` | Temporary directory for Immich-Go operations |

## Quick Examples

```bash
# Upload from local folder (default mode)
immich-go upload --server=http://localhost:2283 --api-key=your-key /photos

# Upload from Google Photos takeout
immich-go upload --google --server=http://localhost:2283 --api-key=your-key /takeout-*.zip

# Archive from Immich server
immich-go archive --from-immich --write-to-folder=/backup --from-server=http://localhost:2283 --from-api-key=your-key

# Stack photos on server
immich-go stack --server=http://localhost:2283 --api-key=your-key --manage-burst=Stack

# Show version
immich-go version
```

## Detailed Command Documentation

- [Upload Command](upload.md) - Comprehensive upload options and sub-commands
- [Archive Command](archive.md) - Export and archival features  
- [Stack Command](stack.md) - Photo organization and stacking
