# upload

Upload photos and videos to your Immich server from various sources.

## Synopsis

```bash
immich-go upload [source-flags] [options] <paths>...
```

## Source Modes

| Flag | Source | Description |
|------|--------|-------------|
| *(default)* | Local filesystem | Upload from folders or ZIP archives |
| `--google` | Google Takeout | Import Google Photos takeout |
| `--icloud` | iCloud export | Import iCloud takeout |
| `--picasa` | Picasa | Parse `.picasa.ini` files for albums |
| `--from-immich` | Immich server | Transfer from another Immich instance |

::: info Mutually Exclusive
`--google`, `--icloud`, and `--from-immich` cannot be combined. `--picasa` can be used with folder mode.
:::

## Server Connection

| Option | Required | Default | Description |
|--------|:--------:|---------|-------------|
| `-s, --server` | Yes | - | Immich server URL |
| `-k, --api-key` | Yes | - | API key |
| `--admin-api-key` | No | - | Admin API key (for job control) |
| `--skip-verify-ssl` | No | false | Skip SSL certificate verification |
| `--client-timeout` | No | 20m | Request timeout |

## Behavior

| Option | Default | Description |
|--------|---------|-------------|
| `--dry-run` | false | Simulate without uploading |
| `--concurrent-tasks` | CPU cores | Parallel uploads (1-20) |
| `--overwrite` | false | Replace existing files |
| `--pause-immich-jobs` | true | Pause server background jobs |
| `--on-errors` | stop | `stop`, `continue`, or error count |

## Output

| Option | Default | Description |
|--------|---------|-------------|
| `--output` | text | `text` or `json` |
| `--api-trace` | false | Log API calls |

## Tagging

| Option | Default | Description |
|--------|---------|-------------|
| `--session-tag` | false | Tag with upload timestamp |
| `--tag` | - | Custom tags (repeatable) |
| `--device-uuid` | $LOCALHOST | Device identifier |

---

## Folder Mode (Default)

Upload from local folders or ZIP archives.

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--recursive` | true | Process subfolders |
| `--date-from-name` | true | Extract date from filename |
| `--ignore-sidecar-files` | false | Skip XMP sidecars |

### File Filtering

| Option | Default | Description |
|--------|---------|-------------|
| `--include-extensions` | all | Only these extensions |
| `--exclude-extensions` | - | Skip these extensions |
| `--include-type` | all | `IMAGE`, `VIDEO`, or `all` |
| `--ban-file` | [defaults](#banned-files) | Exclude by pattern |
| `--date-range` | - | Filter by date |

### Albums

| Option | Default | Description |
|--------|---------|-------------|
| `--folder-as-album` | NONE | `FOLDER`, `PATH`, or `NONE` |
| `--folder-as-tags` | false | Use folders as tags |
| `--into-album` | - | Put all in this album |
| `--album-path-joiner` | " / " | Separator for PATH mode |

### Stacking

| Option | Values | Description |
|--------|--------|-------------|
| `--manage-raw-jpeg` | NoStack, KeepRaw, KeepJPG, StackCoverRaw, StackCoverJPG | RAW+JPEG handling |
| `--manage-heic-jpeg` | NoStack, KeepHeic, KeepJPG, StackCoverHeic, StackCoverJPG | HEIC+JPEG handling |
| `--manage-burst` | NoStack, Stack, StackKeepRaw, StackKeepJPEG | Burst handling |
| `--manage-epson-fastfoto` | false | Epson FastFoto scans |

### Examples

```bash
# Basic upload
immich-go upload --server=http://localhost:2283 --api-key=key /photos

# Create albums from folders
immich-go upload --folder-as-album=FOLDER --server=... --api-key=... /photos

# Stack RAW+JPEG
immich-go upload --manage-raw-jpeg=StackCoverRaw --server=... --api-key=... /photos

# Filter by date and type
immich-go upload --date-range=2023 --include-type=IMAGE --server=... --api-key=... /photos

# JSON output
immich-go upload --output=json --server=... --api-key=... /photos
```

---

## Google Photos Mode

Upload from Google Photos Takeout archives.

```bash
immich-go upload --google [options] <takeout-path>
```

The path can be:
- One or more `takeout-*.zip` files
- A decompressed takeout folder
- A directory containing takeout zip files (auto-detected)

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--include-unmatched` | false | Import files without JSON metadata |
| `--include-archived` | true | Import archived photos |
| `--include-trashed` | false | Import trashed photos |
| `--include-partner` | true | Import partner's photos |
| `--sync-albums` | true | Create matching albums |
| `--include-untitled-albums` | false | Include untitled albums |
| `--from-album-name` | - | Import only this album |
| `--partner-shared-album` | - | Album for partner photos |
| `--takeout-tag` | true | Tag with takeout timestamp |
| `--people-tag` | true | Tag with people names |

### Examples

```bash
# Basic import
immich-go upload --google --server=... --api-key=... /path/to/takeout-*.zip

# Import from folder of zips
immich-go upload --google --server=... --api-key=... /path/to/takeout-folder

# Specific album only
immich-go upload --google --from-album-name="Vacation" --server=... --api-key=... /takeout
```

---

## iCloud Mode

Upload from iCloud exports.

```bash
immich-go upload --icloud [options] <icloud-path>
```

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--memories` | false | Import memories as albums |

### Example

```bash
immich-go upload --icloud --memories --server=... --api-key=... /icloud-export
```

---

## Immich-to-Immich Mode

Transfer between Immich servers.

```bash
immich-go upload --from-immich [source-options] [destination-options]
```

::: info No Path Argument
This mode doesn't accept paths. The source is `--from-server`.
:::

### Source Connection

| Option | Description |
|--------|-------------|
| `--from-server` | Source server URL |
| `--from-api-key` | Source API key |
| `--from-client-timeout` | Source timeout |
| `--from-skip-verify-ssl` | Skip source SSL check |

### Source Filtering

| Option | Description |
|--------|-------------|
| `--from-date-range` | Date range filter |
| `--from-albums` | Filter by album (repeatable) |
| `--from-tags` | Filter by tag |
| `--from-people` | Filter by person |
| `--from-archived` | Include archived |
| `--from-favorite` | Include favorites only |
| `--from-trash` | Include trashed |
| `--from-no-album` | Assets not in albums |
| `--from-minimal-rating` | Minimum rating (1-5) |
| `--from-partners` | Include partner's assets |
| `--from-make` | Filter by camera make |
| `--from-model` | Filter by camera model |
| `--from-country` | Filter by country |
| `--from-state` | Filter by state |
| `--from-city` | Filter by city |

### Example

```bash
immich-go upload --from-immich \
  --from-server=http://old:2283 --from-api-key=old-key \
  --server=http://new:2283 --api-key=new-key
```

---

## Banned Files

By default, these patterns are excluded:

- `.DS_Store`, `/._*` - macOS system files
- `.Spotlight-V100/` - Spotlight index
- `@eaDir/`, `@__thumb/` - Synology thumbnails
- `SYNOFILE_THUMB_*.*` - Synology thumbs
- `thumbnails/` - Generic thumbnails
- `Lightroom Catalog/` - Lightroom data
- `.photostructure/` - PhotoStructure data
- `Recently Deleted/` - Trash folders

Add more with `--ban-file`:

```bash
immich-go upload --ban-file="*.tmp" --ban-file="cache/" ...
```

Patterns ending with `/` match directories.

---

## JSON Output

With `--output=json`, output is JSON Lines (JSONL) to stdout, logs to stderr.

### Progress

```json
{
  "type": "progress",
  "timestamp": "2024-01-15T10:30:00Z",
  "immich_read_pct": 45,
  "assets_found": 234,
  "upload_errors": 0,
  "uploaded": 180
}
```

### Summary

```json
{
  "type": "summary",
  "status": "success",
  "exit_code": 0,
  "counters": {
    "total": 300,
    "duplicates": 50,
    "uploaded": 225,
    "errors": 2
  },
  "duration_seconds": 120.5
}
```

### Shell Integration

```bash
# Save to file
immich-go upload --output=json ... > upload.jsonl

# Process with jq
immich-go upload --output=json ... | jq 'select(.type == "summary")'

# Separate logs from data
immich-go upload --output=json ... > data.jsonl 2> errors.log
```
