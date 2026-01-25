# Configuration

Immich-Go is configured through **CLI flags** and **environment variables**. Configuration files are not supported.

## Configuration Methods

### CLI Flags

All options can be set via command-line flags:

```bash
immich-go upload \
  --server=http://localhost:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=8 \
  /path/to/photos
```

### Environment Variables

All flags can also be set via environment variables. The variable name is derived from the flag name:
- Prefix: `IMMICH_GO_`
- Replace `-` with `_`
- Convert to uppercase

For example:
- `--server` → `IMMICH_GO_SERVER`
- `--api-key` → `IMMICH_GO_API_KEY`
- `--concurrent-tasks` → `IMMICH_GO_CONCURRENT_TASKS`

```bash
export IMMICH_GO_SERVER="http://localhost:2283"
export IMMICH_GO_API_KEY="your-api-key"
immich-go upload /path/to/photos
```

## Common Options

### Global Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--log-level` | `IMMICH_GO_LOG_LEVEL` | `INFO` | Log level: DEBUG, INFO, WARN, ERROR |
| `--dry-run` | `IMMICH_GO_DRY_RUN` | `false` | Simulate without making changes |
| `--concurrent-tasks` | `IMMICH_GO_CONCURRENT_TASKS` | CPU cores | Number of parallel operations |
| `--on-errors` | `IMMICH_GO_ON_ERRORS` | `stop` | Error handling: `stop`, `continue`, or number |

### Server Connection

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--server` | `IMMICH_GO_SERVER` | Immich server URL |
| `--api-key` | `IMMICH_GO_API_KEY` | API key for authentication |
| `--admin-api-key` | `IMMICH_GO_ADMIN_API_KEY` | Admin API key (for job control) |
| `--client-timeout` | `IMMICH_GO_CLIENT_TIMEOUT` | Request timeout (default: 20m) |
| `--skip-verify-ssl` | `IMMICH_GO_SKIP_VERIFY_SSL` | Skip SSL verification |

### Upload Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--overwrite` | `IMMICH_GO_OVERWRITE` | `false` | Replace existing files |
| `--session-tag` | `IMMICH_GO_SESSION_TAG` | `false` | Tag with session timestamp |
| `--tag` | `IMMICH_GO_TAG` | - | Add custom tags |
| `--pause-immich-jobs` | `IMMICH_GO_PAUSE_IMMICH_JOBS` | `true` | Pause server jobs during upload |
| `--output` | `IMMICH_GO_OUTPUT` | `text` | Output format: `text` or `json` |

### Source Mode Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--google` | `IMMICH_GO_GOOGLE` | Import from Google Photos takeout |
| `--icloud` | `IMMICH_GO_ICLOUD` | Import from iCloud takeout |
| `--picasa` | `IMMICH_GO_PICASA` | Enable Picasa album parsing |
| `--from-immich` | `IMMICH_GO_FROM_IMMICH` | Transfer from another Immich server |

### File Filtering

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--date-range` | `IMMICH_GO_DATE_RANGE` | Filter by date range |
| `--include-extensions` | `IMMICH_GO_INCLUDE_EXTENSIONS` | Include only these extensions |
| `--exclude-extensions` | `IMMICH_GO_EXCLUDE_EXTENSIONS` | Exclude these extensions |
| `--include-type` | `IMMICH_GO_INCLUDE_TYPE` | Filter by type: IMAGE, VIDEO |
| `--ban-file` | `IMMICH_GO_BAN_FILE` | Exclude files by pattern |

### Album and Tag Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--folder-as-album` | `IMMICH_GO_FOLDER_AS_ALBUM` | `NONE` | Create albums: FOLDER, PATH, NONE |
| `--folder-as-tags` | `IMMICH_GO_FOLDER_AS_TAGS` | `false` | Use folders as tags |
| `--into-album` | `IMMICH_GO_INTO_ALBUM` | - | Put all photos in this album |
| `--album-path-joiner` | `IMMICH_GO_ALBUM_PATH_JOINER` | ` / ` | Separator for album paths |

### Stacking Options

| Flag | Environment Variable | Values | Description |
|------|---------------------|--------|-------------|
| `--manage-raw-jpeg` | `IMMICH_GO_MANAGE_RAW_JPEG` | `NoStack`, `KeepRaw`, `KeepJPG`, `StackCoverRaw`, `StackCoverJPG` | RAW+JPEG handling |
| `--manage-heic-jpeg` | `IMMICH_GO_MANAGE_HEIC_JPEG` | `NoStack`, `KeepHeic`, `KeepJPG`, `StackCoverHeic`, `StackCoverJPG` | HEIC+JPEG handling |
| `--manage-burst` | `IMMICH_GO_MANAGE_BURST` | `NoStack`, `Stack`, `StackKeepRaw`, `StackKeepJPEG` | Burst photo handling |

## Example Configurations

### Using Environment Variables

```bash
# ~/.bashrc or ~/.zshrc
export IMMICH_GO_SERVER="http://localhost:2283"
export IMMICH_GO_API_KEY="your-api-key"
export IMMICH_GO_CONCURRENT_TASKS="8"
export IMMICH_GO_MANAGE_RAW_JPEG="StackCoverRaw"
export IMMICH_GO_PAUSE_IMMICH_JOBS="true"
```

Then run commands without repeating connection options:

```bash
immich-go upload /photos
immich-go upload --google /takeout-*.zip
immich-go stack --manage-burst=Stack
```

### Wrapper Script

```bash
#!/bin/bash
# immich-upload.sh - Wrapper with common settings

export IMMICH_GO_SERVER="http://localhost:2283"
export IMMICH_GO_API_KEY="$(cat ~/.config/immich-go/api-key)"

immich-go upload \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  --session-tag \
  "$@"
```

## Priority Order

When the same option is set in multiple places, the priority is:
1. **CLI flags** (highest priority)
2. **Environment variables** (lower priority)

CLI flags always override environment variables.

## See Also

- [Command Reference](commands/) - Detailed command options
- [Examples](examples.md) - Practical usage examples
- [Environment Variables](environment.md) - Complete environment variable reference
