# Server-to-Server Migration

Transfer photos between Immich servers without downloading to your local machine.

## Basic Migration

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --server=http://new-server:2283 \
  --api-key=new-api-key
```

::: info No Path Required
Unlike other upload modes, `--from-immich` doesn't take a path argument. The source is the `--from-server`.
:::

## Source Filtering

### By Date Range

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --from-date-range=2023-01-01,2023-12-31 \
  --server=http://new-server:2283 \
  --api-key=new-api-key
```

### By Album

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --from-albums="Family Photos" \
  --from-albums="Travel" \
  --server=http://new-server:2283 \
  --api-key=new-api-key
```

### By Rating

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --from-minimal-rating=3 \
  --server=http://new-server:2283 \
  --api-key=new-api-key
```

### Favorites Only

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --from-favorite=true \
  --server=http://new-server:2283 \
  --api-key=new-api-key
```

## All Filtering Options

| Flag | Description |
|------|-------------|
| `--from-date-range` | Date range (e.g., `2023` or `2023-01-01,2023-06-30`) |
| `--from-albums` | Filter by album name (repeatable) |
| `--from-tags` | Filter by tag |
| `--from-people` | Filter by person name |
| `--from-archived` | Include archived assets |
| `--from-favorite` | Filter by favorite status |
| `--from-trash` | Include trashed assets |
| `--from-no-album` | Include assets not in any album |
| `--from-minimal-rating` | Minimum rating (1-5) |
| `--from-partners` | Include partner's shared assets |
| `--from-make` | Filter by camera make |
| `--from-model` | Filter by camera model |
| `--from-country` | Filter by country |
| `--from-state` | Filter by state |
| `--from-city` | Filter by city |

## Connection Options

Both source and destination servers have their own connection settings:

| Source (from) | Destination | Description |
|---------------|-------------|-------------|
| `--from-server` | `--server` | Server URL |
| `--from-api-key` | `--api-key` | API key |
| `--from-client-timeout` | `--client-timeout` | Request timeout |
| `--from-skip-verify-ssl` | `--skip-verify-ssl` | Skip SSL verification |

## Performance Tips

For large migrations:

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --server=http://new-server:2283 \
  --api-key=new-api-key \
  --concurrent-tasks=4 \
  --on-errors=continue \
  --session-tag
```

## Dry Run

Preview what would be transferred:

```bash
immich-go upload --from-immich \
  --from-server=http://old-server:2283 \
  --from-api-key=old-api-key \
  --server=http://new-server:2283 \
  --api-key=new-api-key \
  --dry-run
```
