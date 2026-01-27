# File Stacking

Stacking groups related photos together. This keeps your library organized without deleting anything.

## What is Stacking?

A stack is a group of related photos shown as one item in Immich, with one photo as the "cover." You can expand a stack to see all photos inside.

Common use cases:
- **RAW + JPEG pairs**: Camera saved both formats
- **HEIC + JPEG pairs**: iPhone saved both formats
- **Burst photos**: Rapid-fire shots from continuous shooting
- **Epson FastFoto scans**: Original, corrected, and back images

## Stacking During Upload

Stack photos as you upload them:

```bash
immich-go upload \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-heic-jpeg=StackCoverJPG \
  --manage-burst=Stack \
  /path/to/photos
```

## Stacking Existing Photos

Use the `stack` command to organize photos already on your server:

```bash
immich-go stack \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack
```

::: tip Test First
Always preview with `--dry-run` before making changes:
```bash
immich-go stack --dry-run ...
```
:::

## RAW + JPEG Options

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `StackCoverRaw` | Stack with RAW as cover |
| `StackCoverJPG` | Stack with JPEG as cover |
| `KeepRaw` | Delete JPEG, keep only RAW |
| `KeepJPG` | Delete RAW, keep only JPEG |

**Recommendations:**
- Photographers who edit RAW: `StackCoverRaw`
- General users who want compatibility: `StackCoverJPG`
- Save space (destructive): `KeepJPG` or `KeepRaw`

## HEIC + JPEG Options

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `StackCoverHeic` | Stack with HEIC as cover |
| `StackCoverJPG` | Stack with JPEG as cover |
| `KeepHeic` | Delete JPEG, keep only HEIC |
| `KeepJPG` | Delete HEIC, keep only JPEG |

**Recommendations:**
- Apple ecosystem: `StackCoverHeic`
- Maximum compatibility: `StackCoverJPG`

## Burst Photo Options

| Value | Behavior |
|-------|----------|
| `NoStack` | Keep separate (default) |
| `Stack` | Stack all burst photos |
| `StackKeepRaw` | Stack, prefer RAW as cover |
| `StackKeepJPEG` | Stack, prefer JPEG as cover |

## How Burst Detection Works

immich-go detects bursts using:

1. **Filename patterns** from specific devices:
   - Huawei: `IMG_*_BURST001_COVER.jpg`
   - Google Pixel: `PXL_*_MOTION-01.COVER.jpg`
   - Samsung: `20231207_101605_001.jpg`
   - Sony Xperia: `DSC_*_BURST*.JPG`
   - Nexus/Nothing: `*_BURST*`

2. **Time-based detection**: Photos taken within 900ms of each other

## Epson FastFoto

For photos scanned with Epson FastFoto:

```bash
immich-go upload \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-epson-fastfoto=true \
  /path/to/scans
```

This stacks:
- `image.jpg` (original scan)
- `image_a.jpg` (auto-corrected)
- `image_b.jpg` (back of photo)

With the corrected version as the cover.

## Best Practices

### 1. One Type at a Time

Handle each type separately to review results:

```bash
# First, stack bursts
immich-go stack --manage-burst=Stack ...

# Then, stack RAW+JPEG
immich-go stack --manage-raw-jpeg=StackCoverRaw ...
```

### 2. Backup First

Before destructive operations (`KeepRaw`, `KeepJPG`), create a backup:

```bash
immich-go archive --from-immich \
  --from-server=http://your-server:2283 \
  --from-api-key=your-api-key \
  --write-to-folder=/backup/before-cleanup
```

### 3. Use Logging

Track what's happening:

```bash
immich-go stack \
  --log-level=DEBUG \
  --manage-burst=Stack \
  --server=... --api-key=... \
  2>&1 | tee stacking.log
```
