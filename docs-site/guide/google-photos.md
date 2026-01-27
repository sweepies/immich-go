# Google Photos Migration

This guide covers migrating your Google Photos library to Immich using immich-go.

## Getting Your Takeout

1. Go to [Google Takeout](https://takeout.google.com/)
2. Deselect all, then select only **Google Photos**
3. Choose your export options:
   - **Delivery method**: Download link via email
   - **Frequency**: Export once
   - **File type**: `.zip`
   - **File size**: 50 GB (larger = fewer parts)
4. Create export and wait for the email
5. Download all `takeout-*.zip` files

::: tip Large Libraries
For libraries with 50,000+ photos, the takeout may be split into many parts. Keep all parts in the same folder.
:::

## Basic Import

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  /path/to/takeout-*.zip
```

If you have a folder of takeout parts, just point to the folder (zip files are detected automatically):

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  /path/to/takeout-folder
```

## What Gets Imported

By default, immich-go imports:

| Content | Imported | Flag to Change |
|---------|----------|----------------|
| Your photos | Yes | - |
| Partner's shared photos | Yes | `--include-partner=false` |
| Archived photos | Yes | `--include-archived=false` |
| Trashed photos | No | `--include-trashed=true` |
| Album organization | Yes | `--sync-albums=false` |
| People tags | Yes | `--people-tag=false` |
| Descriptions & GPS | Yes | - |

## Recommended Settings

### Large Collections (100k+ photos)

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --concurrent-tasks=4 \
  --client-timeout=60m \
  --pause-immich-jobs=true \
  --on-errors=continue \
  --session-tag \
  /path/to/takeout-*.zip
```

- **4 concurrent tasks**: Prevents overwhelming the server
- **60-minute timeout**: Handles large video files
- **Pause jobs**: Speeds up import by pausing thumbnailing
- **Continue on errors**: Don't stop for individual failures
- **Session tag**: Track this import batch

### Medium Collections (10k-100k)

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  /path/to/takeout-*.zip
```

### Import Specific Album Only

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --from-album-name="Vacation 2023" \
  /path/to/takeout-*.zip
```

## Handling RAW+JPEG and Bursts

Google Photos takeouts often include both RAW and JPEG versions, plus burst sequences. Stack them:

```bash
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-raw-jpeg=StackCoverRaw \
  --manage-burst=Stack \
  /path/to/takeout-*.zip
```

| Option | Effect |
|--------|--------|
| `--manage-raw-jpeg=StackCoverRaw` | Stack RAW+JPEG with RAW as cover |
| `--manage-raw-jpeg=StackCoverJPG` | Stack RAW+JPEG with JPEG as cover |
| `--manage-burst=Stack` | Stack burst sequences |

## Troubleshooting

### Files Without Metadata

Some files may not have matching JSON metadata. By default, these are skipped. To import them anyway:

```bash
immich-go upload --google \
  --include-unmatched \
  ...
```

### Duplicate Photos

immich-go detects duplicates using checksums. If you see many "already exists" messages, that's normal—it means you're safely resuming or your photos are already on the server.

### Checking Progress

Use JSON output to monitor progress programmatically:

```bash
immich-go upload --google \
  --output=json \
  --server=... --api-key=... \
  /path/to/takeout-*.zip
```

Progress updates are JSON lines:

```json
{"type":"progress","assets_found":5000,"uploaded":3500,"upload_errors":2}
```

### Resume After Interruption

Just run the same command again. Duplicates are detected and skipped.

## After Import

1. **Review in Immich**: Check that albums and metadata look correct
2. **Run stacking** (if not done during upload):
   ```bash
   immich-go stack \
     --server=http://your-server:2283 \
     --api-key=your-api-key \
     --manage-burst=Stack \
     --manage-raw-jpeg=StackCoverRaw
   ```
3. **Delete takeout files** once you've verified the import
