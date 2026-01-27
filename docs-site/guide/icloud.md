# iCloud Migration

Import your iCloud Photos library to Immich.

## Getting Your iCloud Export

1. Go to [privacy.apple.com](https://privacy.apple.com/)
2. Sign in with your Apple ID
3. Request a copy of your data
4. Select **iCloud Photos**
5. Wait for the export (can take days for large libraries)
6. Download all parts

## Basic Import

```bash
immich-go upload --icloud \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  /path/to/icloud-export
```

## Including Memories

iCloud Memories can be imported as albums:

```bash
immich-go upload --icloud \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --memories \
  /path/to/icloud-export
```

## Handling HEIC+JPEG Pairs

iPhones often save both HEIC and JPEG versions. Stack them:

```bash
immich-go upload --icloud \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --manage-heic-jpeg=StackCoverJPG \
  /path/to/icloud-export
```

| Option | Effect |
|--------|--------|
| `StackCoverJPG` | Stack with JPEG as cover (most compatible) |
| `StackCoverHeic` | Stack with HEIC as cover (smaller file) |
| `KeepJPG` | Keep only JPEG, discard HEIC |
| `KeepHeic` | Keep only HEIC, discard JPEG |

## Full Example

```bash
immich-go upload --icloud \
  --server=http://your-server:2283 \
  --api-key=your-api-key \
  --memories \
  --manage-heic-jpeg=StackCoverJPG \
  --manage-burst=Stack \
  --session-tag \
  /path/to/icloud-export
```
