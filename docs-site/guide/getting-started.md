# Getting Started

immich-go is a command-line tool for uploading photos and videos to your [Immich](https://immich.app/) server. It handles large collections efficiently and supports multiple sources including Google Photos Takeout, iCloud exports, and local folders.

::: warning Early Version
This is an early version, not yet extensively tested. Keep a backup copy of your files for safety.
:::

## Quick Start

### 1. Download

Download the latest release for your platform from the [releases page](https://github.com/sweepies/immich-go/releases/latest).

| Platform | Download |
|----------|----------|
| Windows | `immich-go_Windows_x86_64.zip` |
| macOS | `immich-go_Darwin_x86_64.tar.gz` |
| Linux | `immich-go_Linux_x86_64.tar.gz` |
| FreeBSD | `immich-go_Freebsd_x86_64.tar.gz` |

### 2. Create an API Key

In Immich, go to **Account Settings > API Keys > New API Key** and create a key with these permissions:

- `asset.read`, `asset.update`, `asset.upload`, `asset.delete`, `asset.download`
- `album.create`, `album.read`, `albumAsset.create`
- `stack.create`, `tag.asset`, `tag.create`
- `server.about`, `user.read`

For pausing jobs during upload (recommended), also add `job.create` and `job.read`.

### 3. Upload Your Photos

```bash
# Upload from a local folder
immich-go upload \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  /path/to/photos

# Upload from Google Photos Takeout
immich-go upload --google \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  /path/to/takeout-*.zip
```

## What's Next?

<div class="vp-card-container">

**Migrating from Google Photos?**  
See the [Google Photos migration guide](/guide/google-photos) for best practices.

**Want to organize your library?**  
Learn about [file stacking](/guide/stacking) for RAW+JPEG pairs and burst photos.

**Setting up automation?**  
Check the [configuration reference](/reference/configuration) for environment variables and JSON output.

</div>

## Commands Overview

| Command | Description |
|---------|-------------|
| [`upload`](/commands/upload) | Upload photos from local folders, Google Photos, iCloud, or another Immich server |
| [`archive`](/commands/archive) | Export photos to a date-organized folder structure with metadata |
| [`stack`](/commands/stack) | Organize existing photos into stacks (RAW+JPEG, bursts, etc.) |
