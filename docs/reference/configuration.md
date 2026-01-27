# Configuration

immich-go is configured via CLI flags and environment variables. There are no configuration files.

## Configuration Methods

### CLI Flags

```bash
immich-go upload \
  --server=http://localhost:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=8 \
  /path/to/photos
```

### Environment Variables

All flags can be set via environment variables:

```bash
export IMMICH_GO_UPLOAD_SERVER="http://localhost:2283"
export IMMICH_GO_UPLOAD_API_KEY="your-api-key"
export IMMICH_GO_CONCURRENT_TASKS="8"

immich-go upload /path/to/photos
```

See the [Environment Variables](/reference/environment) reference for the complete list.

## Priority

CLI flags override environment variables.

## Global Options

These apply to all commands.

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--log-level` | `IMMICH_GO_LOG_LEVEL` | INFO | DEBUG, INFO, WARN, ERROR |
| `--dry-run` | `IMMICH_GO_DRY_RUN` | false | Simulate without changes |
| `--concurrent-tasks` | `IMMICH_GO_CONCURRENT_TASKS` | CPU cores | Parallel operations (1-20) |
| `--on-errors` | `IMMICH_GO_ON_ERRORS` | stop | stop, continue, or number |
| `--output` | `IMMICH_GO_OUTPUT` | text | text or json |

## Server Connection

Command-scoped options. The environment variable includes the command name.

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--server` | `IMMICH_GO_UPLOAD_SERVER` | Server URL |
| `--api-key` | `IMMICH_GO_UPLOAD_API_KEY` | API key |
| `--admin-api-key` | `IMMICH_GO_UPLOAD_ADMIN_API_KEY` | Admin API key |
| `--client-timeout` | `IMMICH_GO_UPLOAD_CLIENT_TIMEOUT` | Request timeout (default: 20m) |
| `--skip-verify-ssl` | `IMMICH_GO_UPLOAD_SKIP_VERIFY_SSL` | Skip SSL verification |
| `--on-errors` | `IMMICH_GO_ON_ERRORS` | Error handling: `stop`, `continue`, or number |

Replace `UPLOAD` with `ARCHIVE` or `STACK` for other commands.

## Upload Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--overwrite` | `IMMICH_GO_UPLOAD_OVERWRITE` | false | Replace existing files |
| `--session-tag` | `IMMICH_GO_UPLOAD_SESSION_TAG` | false | Tag with session timestamp |
| `--tag` | `IMMICH_GO_UPLOAD_TAG` | - | Custom tags |
| `--pause-immich-jobs` | `IMMICH_GO_UPLOAD_PAUSE_IMMICH_JOBS` | true | Pause server jobs |
| `--no-resume-jobs` | `IMMICH_GO_UPLOAD_NO_RESUME_JOBS` | false | Do not resume jobs (testing) |
| `--include-unmatched` | `IMMICH_GO_UPLOAD_INCLUDE_UNMATCHED` | false | Upload even if JSON match fails |


## Source Modes

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--google` | `IMMICH_GO_UPLOAD_GOOGLE` | Google Photos takeout |
| `--icloud` | `IMMICH_GO_UPLOAD_ICLOUD` | iCloud export |
| `--picasa` | `IMMICH_GO_UPLOAD_PICASA` | Picasa mode |
| `--from-immich` | `IMMICH_GO_UPLOAD_FROM_IMMICH` | Immich-to-Immich |

## File Filtering

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--date-range` | `IMMICH_GO_UPLOAD_DATE_RANGE` | Date filter |
| `--include-extensions` | `IMMICH_GO_UPLOAD_INCLUDE_EXTENSIONS` | Only these extensions |
| `--exclude-extensions` | `IMMICH_GO_UPLOAD_EXCLUDE_EXTENSIONS` | Skip these extensions |
| `--include-type` | `IMMICH_GO_UPLOAD_INCLUDE_TYPE` | IMAGE or VIDEO |
| `--ban-file` | `IMMICH_GO_UPLOAD_BAN_FILE` | Exclude patterns |

## Album Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--folder-as-album` | `IMMICH_GO_UPLOAD_FOLDER_AS_ALBUM` | NONE | FOLDER, PATH, NONE |
| `--folder-as-tags` | `IMMICH_GO_UPLOAD_FOLDER_AS_TAGS` | false | Folders as tags |
| `--into-album` | `IMMICH_GO_UPLOAD_INTO_ALBUM` | - | Single album |
| `--album-path-joiner` | `IMMICH_GO_UPLOAD_ALBUM_PATH_JOINER` | " / " | Path separator |

## Stacking Options

| Flag | Environment Variable | Values |
|------|---------------------|--------|
| `--manage-raw-jpeg` | `IMMICH_GO_UPLOAD_MANAGE_RAW_JPEG` | NoStack, KeepRaw, KeepJPG, StackCoverRaw, StackCoverJPG |
| `--manage-heic-jpeg` | `IMMICH_GO_UPLOAD_MANAGE_HEIC_JPEG` | NoStack, KeepHeic, KeepJPG, StackCoverHeic, StackCoverJPG |
| `--manage-burst` | `IMMICH_GO_UPLOAD_MANAGE_BURST` | NoStack, Stack, StackKeepRaw, StackKeepJPEG |

## Example Configurations

### Shell Profile

```bash
# ~/.bashrc or ~/.zshrc
export IMMICH_GO_UPLOAD_SERVER="http://localhost:2283"
export IMMICH_GO_UPLOAD_API_KEY="your-api-key"
export IMMICH_GO_CONCURRENT_TASKS="8"
export IMMICH_GO_UPLOAD_MANAGE_RAW_JPEG="StackCoverRaw"
export IMMICH_GO_UPLOAD_PAUSE_IMMICH_JOBS="true"
```

Then run without repeating options:

```bash
immich-go upload /photos
immich-go upload --google /takeout-*.zip
```

### Wrapper Script

```bash
#!/bin/bash
# immich-upload.sh

export IMMICH_GO_UPLOAD_SERVER="http://localhost:2283"
export IMMICH_GO_UPLOAD_API_KEY="$(cat ~/.config/immich-go/api-key)"

immich-go upload \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  --session-tag \
  "$@"
```

### Systemd Environment File

```bash
# /etc/immich-go.env
IMMICH_GO_UPLOAD_SERVER=http://localhost:2283
IMMICH_GO_UPLOAD_API_KEY=your-api-key
IMMICH_GO_CONCURRENT_TASKS=4
```

## Date Range Formats

| Format | Example | Description |
|--------|---------|-------------|
| Year | `2023` | Entire year |
| Month | `2023-07` | Entire month |
| Day | `2023-07-15` | Single day |
| Range | `2023-01-15,2023-03-15` | Start to end |
