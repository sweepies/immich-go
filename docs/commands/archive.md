# archive

Export photos to a date-organized folder structure with metadata.

## Synopsis

```bash
immich-go archive [source-flags] --write-to-folder=<destination> [options] <paths>...
```

## Output Structure

Photos are organized by year and month:

```
destination/
├── 2022/
│   ├── 2022-01/
│   │   ├── photo.jpg
│   │   └── photo.jpg.JSON
│   └── 2022-02/
├── 2023/
│   ├── 2023-03/
│   └── 2023-04/
└── 2024/
```

Each photo gets a `.JSON` sidecar with metadata.

## Required Options

| Option | Description |
|--------|-------------|
| `--write-to-folder` | Destination folder |

## Source Modes

| Flag | Source | Description |
|------|--------|-------------|
| *(default)* | Local filesystem | Archive from folders |
| `--google` | Google Takeout | Archive from takeout |
| `--icloud` | iCloud export | Archive from iCloud |
| `--picasa` | Picasa | Parse Picasa metadata |
| `--from-immich` | Immich server | Archive from server |

## Metadata Files

Each photo gets a `.JSON` file containing:

```json
{
  "fileName": "example.jpg",
  "latitude": 37.7749,
  "longitude": -122.4194,
  "dateTaken": "2023-10-01T12:34:56Z",
  "description": "Golden Gate Bridge",
  "albums": [
    {
      "title": "San Francisco Trip",
      "description": "Photos from my trip"
    }
  ],
  "tags": [
    { "value": "USA/California/San Francisco" }
  ],
  "rating": 5,
  "trashed": false,
  "archived": false,
  "favorited": true,
  "fromPartner": false
}
```

## Examples

### Archive from Immich Server

```bash
# Complete backup
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --write-to-folder=/backup/photos

# Specific year
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --from-date-range=2023 \
  --write-to-folder=/backup/2023

# Specific album
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --from-albums="Family" \
  --write-to-folder=/backup/family
```

### Archive Google Photos Takeout

```bash
# Reorganize takeout by date
immich-go archive --google \
  --write-to-folder=/organized-photos \
  /path/to/takeout-*.zip
```

### Reorganize Local Folders

```bash
# Transform messy folders into date structure
immich-go archive \
  --write-to-folder=/organized \
  /messy/photo/folders
```

## Use Cases

### 1. Server Backup

Create a portable backup of your Immich library:

```bash
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --write-to-folder=/backup/immich-$(date +%Y-%m-%d)
```

### 2. Incremental Backup

Backup recent photos (last 30 days):

```bash
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --from-date-range=$(date -d '30 days ago' '+%Y-%m-%d'),$(date '+%Y-%m-%d') \
  --write-to-folder=/backup/recent
```

### 3. Yearly Archives

```bash
for year in 2020 2021 2022 2023 2024; do
  immich-go archive --from-immich \
    --from-server=http://localhost:2283 \
    --from-api-key=your-key \
    --from-date-range=$year \
    --write-to-folder=/backup/immich-$year
done
```

### 4. Migration Preparation

Export for migration to another system:

```bash
immich-go archive --from-immich \
  --from-server=http://localhost:2283 \
  --from-api-key=your-key \
  --write-to-folder=/migration-ready
```

## Important Notes

- **Incremental**: Running again adds new photos without duplicating
- **Metadata preserved**: JSON files capture all metadata
- **Cross-platform**: Archived photos work with any system
- **No server changes**: Archive is read-only for the source

## Options Reference

### Connection Options (for `--from-immich`)

Same as [upload command](/commands/upload#server-connection).

### Filtering Options

Same filtering options as the corresponding upload modes:
- `--date-range` / `--from-date-range`
- `--from-albums`
- `--include-type`, `--include-extensions`
