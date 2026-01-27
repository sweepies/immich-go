# Environment Variables

Complete reference of all environment variables. Variables follow the pattern `IMMICH_GO_<COMMAND>_<FLAG>`.

## Naming Convention

| Flag | Environment Variable |
|------|---------------------|
| `--log-level` (global) | `IMMICH_GO_LOG_LEVEL` |
| `--server` (upload) | `IMMICH_GO_UPLOAD_SERVER` |
| `--from-api-key` (upload) | `IMMICH_GO_UPLOAD_FROM_API_KEY` |

Rules:
1. Prefix: `IMMICH_GO_`
2. Global flags: `IMMICH_GO_<FLAG>`
3. Command flags: `IMMICH_GO_<COMMAND>_<FLAG>`
4. Replace `-` with `_`
5. Uppercase

## Global Variables

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_CONCURRENT_TASKS` | `--concurrent-tasks` | 10 | Parallel tasks (1-20) |
| `IMMICH_GO_DRY_RUN` | `--dry-run` | false | Simulate without changes |
| `IMMICH_GO_LOG_LEVEL` | `--log-level` | INFO | DEBUG, INFO, WARN, ERROR |
| `IMMICH_GO_ON_ERRORS` | `--on-errors` | stop | stop, continue, or number |
| `IMMICH_GO_OUTPUT` | `--output` | text | text or json |

## Upload Variables

### Server Connection

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_SERVER` | `--server` | - | Server URL |
| `IMMICH_GO_UPLOAD_API_KEY` | `--api-key` | - | API key |
| `IMMICH_GO_UPLOAD_ADMIN_API_KEY` | `--admin-api-key` | - | Admin API key |
| `IMMICH_GO_UPLOAD_CLIENT_TIMEOUT` | `--client-timeout` | 20m | Request timeout |
| `IMMICH_GO_UPLOAD_SKIP_VERIFY_SSL` | `--skip-verify-ssl` | false | Skip SSL check |
| `IMMICH_GO_UPLOAD_API_TRACE` | `--api-trace` | false | Log API calls |

### Upload Behavior

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_DRY_RUN` | `--dry-run` | false | Simulate |
| `IMMICH_GO_UPLOAD_OVERWRITE` | `--overwrite` | false | Replace existing |
| `IMMICH_GO_UPLOAD_PAUSE_IMMICH_JOBS` | `--pause-immich-jobs` | true | Pause server jobs |
| `IMMICH_GO_UPLOAD_NO_RESUME_JOBS` | `--no-resume-jobs` | false | Don't resume jobs |
| `IMMICH_GO_UPLOAD_DEVICE_UUID` | `--device-uuid` | HOSTNAME | Device identifier |
| `IMMICH_GO_UPLOAD_TIME_ZONE` | `--time-zone` | - | Override timezone |

### Source Modes

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_GOOGLE` | `--google` | false | Google Photos mode |
| `IMMICH_GO_UPLOAD_ICLOUD` | `--icloud` | false | iCloud mode |
| `IMMICH_GO_UPLOAD_PICASA` | `--picasa` | false | Picasa mode |
| `IMMICH_GO_UPLOAD_FROM_IMMICH` | `--from-immich` | false | Immich-to-Immich |

### File Handling

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_RECURSIVE` | `--recursive` | true | Process subfolders |
| `IMMICH_GO_UPLOAD_DATE_FROM_NAME` | `--date-from-name` | true | Extract date from filename |
| `IMMICH_GO_UPLOAD_IGNORE_SIDECAR_FILES` | `--ignore-sidecar-files` | false | Skip XMP files |
| `IMMICH_GO_UPLOAD_BAN_FILE` | `--ban-file` | [defaults] | Exclude patterns |

### Filtering

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_DATE_RANGE` | `--date-range` | - | Date filter |
| `IMMICH_GO_UPLOAD_INCLUDE_EXTENSIONS` | `--include-extensions` | all | Only these extensions |
| `IMMICH_GO_UPLOAD_EXCLUDE_EXTENSIONS` | `--exclude-extensions` | - | Skip these extensions |
| `IMMICH_GO_UPLOAD_INCLUDE_TYPE` | `--include-type` | all | IMAGE or VIDEO |

### Albums and Tags

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_FOLDER_AS_ALBUM` | `--folder-as-album` | NONE | FOLDER, PATH, NONE |
| `IMMICH_GO_UPLOAD_FOLDER_AS_TAGS` | `--folder-as-tags` | false | Folders as tags |
| `IMMICH_GO_UPLOAD_INTO_ALBUM` | `--into-album` | - | Put all in album |
| `IMMICH_GO_UPLOAD_ALBUM_PATH_JOINER` | `--album-path-joiner` | " / " | Path separator |
| `IMMICH_GO_UPLOAD_ALBUM_PICASA` | `--album-picasa` | false | Use Picasa albums |
| `IMMICH_GO_UPLOAD_TAG` | `--tag` | [] | Custom tags |
| `IMMICH_GO_UPLOAD_SESSION_TAG` | `--session-tag` | false | Session timestamp tag |

### Stacking

| Variable | Flag | Default | Values |
|----------|------|---------|--------|
| `IMMICH_GO_UPLOAD_MANAGE_RAW_JPEG` | `--manage-raw-jpeg` | NoStack | NoStack, KeepRaw, KeepJPG, StackCoverRaw, StackCoverJPG |
| `IMMICH_GO_UPLOAD_MANAGE_HEIC_JPEG` | `--manage-heic-jpeg` | NoStack | NoStack, KeepHeic, KeepJPG, StackCoverHeic, StackCoverJPG |
| `IMMICH_GO_UPLOAD_MANAGE_BURST` | `--manage-burst` | NoStack | NoStack, Stack, StackKeepRaw, StackKeepJPEG |
| `IMMICH_GO_UPLOAD_MANAGE_EPSON_FASTFOTO` | `--manage-epson-fastfoto` | false | true/false |

### Google Photos Options

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_INCLUDE_UNMATCHED` | `--include-unmatched` | false | Files without JSON |
| `IMMICH_GO_UPLOAD_INCLUDE_ARCHIVED` | `--include-archived` | true | Archived photos |
| `IMMICH_GO_UPLOAD_INCLUDE_TRASHED` | `--include-trashed` | false | Trashed photos |
| `IMMICH_GO_UPLOAD_INCLUDE_PARTNER` | `--include-partner` | true | Partner's photos |
| `IMMICH_GO_UPLOAD_SYNC_ALBUMS` | `--sync-albums` | true | Create matching albums |
| `IMMICH_GO_UPLOAD_INCLUDE_UNTITLED_ALBUMS` | `--include-untitled-albums` | false | Untitled albums |
| `IMMICH_GO_UPLOAD_FROM_ALBUM_NAME` | `--from-album-name` | - | Specific album only |
| `IMMICH_GO_UPLOAD_PARTNER_SHARED_ALBUM` | `--partner-shared-album` | - | Album for partner photos |
| `IMMICH_GO_UPLOAD_TAKEOUT_TAG` | `--takeout-tag` | true | Takeout timestamp tag |
| `IMMICH_GO_UPLOAD_PEOPLE_TAG` | `--people-tag` | true | People name tags |

### iCloud Options

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_MEMORIES` | `--memories` | false | Import memories as albums |

### Immich-to-Immich Source

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_UPLOAD_FROM_SERVER` | `--from-server` | - | Source server URL |
| `IMMICH_GO_UPLOAD_FROM_API_KEY` | `--from-api-key` | - | Source API key |
| `IMMICH_GO_UPLOAD_FROM_ADMIN_API_KEY` | `--from-admin-api-key` | - | Source admin key |
| `IMMICH_GO_UPLOAD_FROM_CLIENT_TIMEOUT` | `--from-client-timeout` | 20m | Source timeout |
| `IMMICH_GO_UPLOAD_FROM_SKIP_VERIFY_SSL` | `--from-skip-verify-ssl` | false | Source SSL check |
| `IMMICH_GO_UPLOAD_FROM_DATE_RANGE` | `--from-date-range` | - | Source date filter |
| `IMMICH_GO_UPLOAD_FROM_ALBUMS` | `--from-albums` | [] | Source albums |
| `IMMICH_GO_UPLOAD_FROM_TAGS` | `--from-tags` | [] | Source tags |
| `IMMICH_GO_UPLOAD_FROM_PEOPLE` | `--from-people` | [] | Source people |
| `IMMICH_GO_UPLOAD_FROM_ARCHIVED` | `--from-archived` | false | Archived only |
| `IMMICH_GO_UPLOAD_FROM_FAVORITE` | `--from-favorite` | false | Favorites only |
| `IMMICH_GO_UPLOAD_FROM_TRASH` | `--from-trash` | false | Trashed only |
| `IMMICH_GO_UPLOAD_FROM_NO_ALBUM` | `--from-no-album` | false | Not in albums |
| `IMMICH_GO_UPLOAD_FROM_MINIMAL_RATING` | `--from-minimal-rating` | 0 | Minimum rating |
| `IMMICH_GO_UPLOAD_FROM_PARTNERS` | `--from-partners` | false | Partner's assets |
| `IMMICH_GO_UPLOAD_FROM_MAKE` | `--from-make` | - | Camera make |
| `IMMICH_GO_UPLOAD_FROM_MODEL` | `--from-model` | - | Camera model |
| `IMMICH_GO_UPLOAD_FROM_COUNTRY` | `--from-country` | - | Country |
| `IMMICH_GO_UPLOAD_FROM_STATE` | `--from-state` | - | State |
| `IMMICH_GO_UPLOAD_FROM_CITY` | `--from-city` | - | City |

## Archive Variables

Same as upload variables, prefixed with `IMMICH_GO_ARCHIVE_` instead of `IMMICH_GO_UPLOAD_`.

Additional:

| Variable | Flag | Description |
|----------|------|-------------|
| `IMMICH_GO_ARCHIVE_WRITE_TO_FOLDER` | `--write-to-folder` | Output folder |

## Stack Variables

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `IMMICH_GO_STACK_SERVER` | `--server` | - | Server URL |
| `IMMICH_GO_STACK_API_KEY` | `--api-key` | - | API key |
| `IMMICH_GO_STACK_ADMIN_API_KEY` | `--admin-api-key` | - | Admin API key |
| `IMMICH_GO_STACK_CLIENT_TIMEOUT` | `--client-timeout` | 20m | Request timeout |
| `IMMICH_GO_STACK_SKIP_VERIFY_SSL` | `--skip-verify-ssl` | false | Skip SSL check |
| `IMMICH_GO_STACK_API_TRACE` | `--api-trace` | false | Log API calls |
| `IMMICH_GO_STACK_DRY_RUN` | `--dry-run` | false | Simulate |
| `IMMICH_GO_STACK_DATE_RANGE` | `--date-range` | - | Date filter |
| `IMMICH_GO_STACK_TIME_ZONE` | `--time-zone` | - | Override timezone |
| `IMMICH_GO_STACK_DEVICE_UUID` | `--device-uuid` | HOSTNAME | Device identifier |
| `IMMICH_GO_STACK_PAUSE_IMMICH_JOBS` | `--pause-immich-jobs` | true | Pause server jobs |
| `IMMICH_GO_STACK_MANAGE_RAW_JPEG` | `--manage-raw-jpeg` | NoStack | RAW+JPEG handling |
| `IMMICH_GO_STACK_MANAGE_HEIC_JPEG` | `--manage-heic-jpeg` | NoStack | HEIC+JPEG handling |
| `IMMICH_GO_STACK_MANAGE_BURST` | `--manage-burst` | NoStack | Burst handling |
| `IMMICH_GO_STACK_MANAGE_EPSON_FASTFOTO` | `--manage-epson-fastfoto` | false | Epson handling |
