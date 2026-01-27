# Performance Tuning

Optimize immich-go for your network and server.

## Key Settings

| Setting | Flag | Default | Description |
|---------|------|---------|-------------|
| Parallel uploads | `--concurrent-tasks` | CPU cores | Number of simultaneous uploads |
| Timeout | `--client-timeout` | 20m | Time before giving up on a request |
| Pause jobs | `--pause-immich-jobs` | true | Pause Immich background jobs |
| Error handling | `--on-errors` | stop | What to do when errors occur |

## Recommended Configurations

### Fast Local Network (Gigabit+)

```bash
immich-go upload \
  --concurrent-tasks=12 \
  --client-timeout=30m \
  --pause-immich-jobs=true \
  ...
```

### Internet Upload

```bash
immich-go upload \
  --concurrent-tasks=4 \
  --client-timeout=60m \
  --on-errors=continue \
  ...
```

### Slow or Unstable Connection

```bash
immich-go upload \
  --concurrent-tasks=1 \
  --client-timeout=120m \
  --on-errors=continue \
  ...
```

### Powerful Server

```bash
immich-go upload \
  --concurrent-tasks=16 \
  --pause-immich-jobs=false \
  ...
```

### NAS / Low-Power Server

```bash
immich-go upload \
  --concurrent-tasks=2 \
  --client-timeout=180m \
  --pause-immich-jobs=true \
  ...
```

## Concurrent Tasks

The `--concurrent-tasks` setting controls parallel uploads:

| Range | Best For |
|-------|----------|
| 1-2 | Slow connections, low-power servers |
| 4-8 | Most users, internet uploads |
| 12-16 | Fast LAN, powerful servers |
| 16+ | Diminishing returns, may reduce reliability |

::: info Network Bound
Performance testing shows immich-go is network-bound, not CPU-bound. More concurrent tasks help up to your bandwidth limit, then provide no benefit.
:::

## Pausing Immich Jobs

By default, immich-go pauses Immich's background jobs (thumbnailing, ML processing) during uploads. This:

- **Speeds up uploads** by reducing server load
- **Uses an admin API key** (requires `job.create` and `job.read` permissions)

To disable:

```bash
immich-go upload --pause-immich-jobs=false ...
```

## Error Handling

| Value | Behavior |
|-------|----------|
| `stop` | Stop on first error (default) |
| `continue` | Skip errors, continue uploading |
| `5` | Allow up to 5 errors, then stop |

For large imports, `continue` is recommended:

```bash
immich-go upload --on-errors=continue ...
```

## Timeouts

Large video files may need longer timeouts:

```bash
# For 4K videos and large RAW files
immich-go upload --client-timeout=60m ...

# For very large files or slow connections
immich-go upload --client-timeout=180m ...
```

## Monitoring Progress

### Text Output (Default)

Progress updates every 5 seconds:

```
Immich read 50%, Assets found: 200, Upload errors: 0, Uploaded 150
```

### JSON Output

For automation and detailed tracking:

```bash
immich-go upload --output=json ... > progress.jsonl 2> errors.log
```

Each line is a JSON object:

```json
{"type":"progress","immich_read_pct":45,"assets_found":234,"uploaded":180}
```

## Background Uploads

Run uploads in the background:

```bash
nohup immich-go upload \
  --server=... --api-key=... \
  /path/to/photos \
  > upload.log 2>&1 &
```

With JSON output:

```bash
nohup immich-go upload \
  --output=json \
  --server=... --api-key=... \
  /path/to/photos \
  > upload.jsonl 2> errors.log &
```

## Resuming Interrupted Uploads

immich-go uses checksums to detect duplicates. If an upload is interrupted:

1. Fix the issue (network, server, etc.)
2. Run the exact same command again
3. Already-uploaded photos are skipped automatically

No special flags needed.
