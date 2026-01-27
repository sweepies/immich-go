# stack

Organize existing photos on your Immich server into stacks.

## Synopsis

```bash
immich-go stack [options]
```

## Purpose

Stacking groups related photos without deleting anything:

- **Burst photos**: Rapid-fire shots
- **RAW + JPEG**: Camera saved both
- **HEIC + JPEG**: iPhone saved both
- **Epson FastFoto**: Scanned photo sets

## Required Options

| Option | Description |
|--------|-------------|
| `-s, --server` | Immich server URL |
| `-k, --api-key` | API key |

## Connection Options

| Option | Default | Description |
|--------|---------|-------------|
| `--skip-verify-ssl` | false | Skip SSL verification |
| `--client-timeout` | 20m | Request timeout |
| `--api-trace` | false | Log API calls |

## Behavior Options

| Option | Default | Description |
|--------|---------|-------------|
| `--dry-run` | false | Preview without changes |
| `--time-zone` | System | Override timezone |

## Stacking Rules

### Burst Photos

```bash
immich-go stack --manage-burst=Stack --server=... --api-key=...
```

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `Stack` | Stack all bursts |
| `StackKeepRaw` | Stack, RAW as cover |
| `StackKeepJPEG` | Stack, JPEG as cover |

**Detected patterns:**

| Device | Pattern |
|--------|---------|
| Huawei | `IMG_*_BURST001_COVER.jpg` |
| Google Pixel | `PXL_*_MOTION-01.COVER.jpg` |
| Samsung | `20231207_101605_001.jpg` |
| Sony Xperia | `DSC_*_BURST*.JPG` |
| Nexus | `*_BURST*_COVER.jpg` |
| Nothing | `*_BURST*` |

Also detects photos taken within 900ms of each other.

### RAW + JPEG

```bash
immich-go stack --manage-raw-jpeg=StackCoverRaw --server=... --api-key=...
```

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `StackCoverRaw` | Stack, RAW as cover |
| `StackCoverJPG` | Stack, JPEG as cover |
| `KeepRaw` | Delete JPEG |
| `KeepJPG` | Delete RAW |

### HEIC + JPEG

```bash
immich-go stack --manage-heic-jpeg=StackCoverJPG --server=... --api-key=...
```

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `StackCoverHeic` | Stack, HEIC as cover |
| `StackCoverJPG` | Stack, JPEG as cover |
| `KeepHeic` | Delete JPEG |
| `KeepJPG` | Delete HEIC |

### Epson FastFoto

```bash
immich-go stack --manage-epson-fastfoto=true --server=... --api-key=...
```

Stacks scanner output:
- `image.jpg` (original)
- `image_a.jpg` (corrected) - becomes cover
- `image_b.jpg` (back of photo)

## Examples

### Preview Changes

```bash
immich-go stack \
  --server=http://localhost:2283 \
  --api-key=your-key \
  --manage-burst=Stack \
  --dry-run
```

### Stack Bursts

```bash
immich-go stack \
  --server=http://localhost:2283 \
  --api-key=your-key \
  --manage-burst=Stack
```

### Stack RAW+JPEG with RAW Cover

```bash
immich-go stack \
  --server=http://localhost:2283 \
  --api-key=your-key \
  --manage-raw-jpeg=StackCoverRaw
```

### Full Organization

```bash
immich-go stack \
  --server=http://localhost:2283 \
  --api-key=your-key \
  --manage-burst=Stack \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-heic-jpeg=StackCoverJPG
```

## Best Practices

### 1. Always Preview First

```bash
immich-go stack --dry-run ...
```

### 2. One Type at a Time

```bash
# First bursts
immich-go stack --manage-burst=Stack ...

# Then RAW+JPEG
immich-go stack --manage-raw-jpeg=StackCoverRaw ...
```

### 3. Backup Before Destructive Operations

Before using `KeepRaw` or `KeepJPG`:

```bash
immich-go archive --from-immich \
  --from-server=... --from-api-key=... \
  --write-to-folder=/backup
```

### 4. Use Debug Logging

```bash
immich-go stack \
  --log-level=DEBUG \
  --manage-burst=Stack \
  --server=... --api-key=... \
  2>&1 | tee stacking.log
```

## Troubleshooting

### Nothing Gets Stacked

- Check that photos have expected timestamps
- Verify filenames match device patterns
- Use `--api-trace` to see server communication
- Try `--dry-run` with `--log-level=DEBUG`

### Unexpected Stacking

- Review detection patterns for your devices
- Time-based detection (900ms) may be too aggressive for some workflows

### Performance Issues

- Increase `--client-timeout` for large libraries
- Process incrementally using archive/upload workflow with date ranges
