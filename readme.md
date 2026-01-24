# immich-go: The Missing Piece

![logo](https://raw.githubusercontent.com/sweepies/immich-go/refs/heads/main/logo.jpg)

**Immich-Go** is an open-source tool designed to streamline uploading large photo collections to your Immich server.

> ℹ️ **This is a fork** of [simulot/immich-go](https://github.com/simulot/immich-go). See [Fork Differences](#-fork-differences) for details about this fork.

> ⚠️ This is an early version, not yet extensively tested<br>
> ⚠️ Keep a backup copy of your files for safety<br>

## 🌟 Key Features

- **Simple Installation**: Single binary executable
- **Multiple Sources**: Upload from Google Photos, iCloud, local folders, zip archives, and other Immich servers
- **Large Collections**: Successfully handles 100,000+ photos
- **Smart Management**: Duplicate detection, burst photo stacking, RAW+JPEG handling
- **Cross-Platform**: Available for Windows, macOS, Linux, and FreeBSD

## 🔀 Fork Differences

This fork of [simulot/immich-go](https://github.com/simulot/immich-go) includes the following modifications:

### New Features

- **JSON output mode** – Use `--output=json` for JSONL progress streaming and structured JSON summary, with logs on stderr
- **Non-interactive mode** – Use `--non-interactive` for line-by-line progress, with auto-detection when stdout isn't a TTY

### Improvements

- **Unix-style output** – Consistent stdout/stderr separation: reports to stdout, logs and progress to stderr
- **Output validation** – Invalid `--output` values now show clear validation errors

### Bug Fixes

- **Duplicate summaries** – Prevented double JSON summaries for upload subcommands
- **Duplicate reports** – Removed duplicate final report output in text mode
- **Error reporting** – JSON summaries now reflect upload failures and write errors

### Breaking Changes

- **Logging flags removed** – The `--log-file` and `--log-type` flags have been removed; use shell redirection instead (e.g., `2>logs.txt`)
- **UI flag removed** – The `--no-ui` flag has been removed; use `--non-interactive` instead

### Release Versioning

- Fork releases use a `-sweepy` version suffix (e.g., `v0.32.0-sweepy`)
- See the [releases page](https://github.com/sweepies/immich-go/releases) for version history

For the upstream project, see [simulot/immich-go](https://github.com/simulot/immich-go).

## 🚀 Quick Start

### 1. Install Immich-Go

Download the pre-built binary for your system from the [GitHub releases page](https://github.com/sweepies/immich-go/releases).

### 2. Basic Usage

```bash
# Upload photos from a local folder
immich-go upload from-folder --server=http://your-ip:2283 --api-key=your-api-key /path/to/your/photos

# Upload Google Photos takeout
immich-go upload from-google-photos --server=http://your-ip:2283 --api-key=your-api-key /path/to/takeout-*.zip

# Archive photos from Immich server
immich-go archive from-immich --server=http://your-ip:2283 --api-key=your-api-key --write-to-folder=/path/to/archive

# JSON output for automation
immich-go upload from-folder --server=http://your-ip:2283 --api-key=your-api-key --output=json /path/to/photos

# Non-interactive mode for scripts
immich-go upload from-folder --server=http://your-ip:2283 --api-key=your-api-key --non-interactive /path/to/photos
```

### 3. Requirements

- A running Immich server with API access
- API key with appropriate permissions ([see full list](docs/installation.md#api-permissions))

## 🙈 Skip System Files

- Use `--ban-file` to exclude junk artifacts. Patterns ending with `/` apply to directories (for example, `--ban-file .Spotlight-V100/`), while patterns without the trailing slash apply to individual files (for example, `--ban-file .DS_Store`).
- Immich-Go ships with sensible defaults that already skip common clutter such as `@eaDir/`, `@__thumb/`, `SYNOFILE_THUMB_*.*`, `Lightroom Catalog/`, `thumbnails/`, `.DS_Store`, `/._*`, `.Spotlight-V100/`, `.photostructure/`, and `Recently Deleted/`.
- Add additional patterns as needed to keep uploads focused on real photos. See the [banned files reference](docs/technical.md#banned-files) for details.

## 📚 Documentation

| Topic | Description |
|-------|-------------|
| [Installation](docs/installation.md) | Detailed installation instructions for all platforms |
| [Commands](docs/commands/) | Complete command reference and options |
| [Configuration](docs/configuration.md) | Configuration options and environment variables |
| [Examples](docs/examples.md) | Common use cases and practical examples |
| [Best Practices](docs/best-practices.md) | Tips for optimal performance and reliability |
| [Technical Details](docs/technical.md) | File processing, metadata handling, and advanced features |
| [Upload Commands Overview](docs/upload-commands-overview.md) | How `immich-go` processes files from different sources |
| [Release Notes](docs/releases/) | Version history and release notes |

## ✨ How immich-go Works

`immich-go` offers a versatile set of commands to handle your photo and video uploads. Whether you're uploading from a simple folder, migrating from a Google Photos Takeout, or transferring assets between Immich servers, the tool provides intelligent features to preserve your metadata and organization.

Here's a brief overview of the main upload commands:

- **`from-folder`**: The basic command for uploading from any local folder. It can create albums from your directory structure and read XMP sidecar files.
- **`from-google-photos`**: A powerful command to migrate from a Google Photos Takeout. It intelligently matches photos with their JSON metadata to preserve albums, descriptions, and locations.
- **`from-immich`**: A server-to-server migration tool that allows you to copy assets between two Immich instances with fine-grained filtering.
- **`from-picasa`**: A specialized version of `from-folder` that automatically reads `.picasa.ini` files to restore your Picasa album organization.
- **`from-icloud`**: Another specialized command that handles the complexity of an iCloud Photos takeout, correctly identifying creation dates and album structures from the included CSV files.

### Leveraging Immich's Features

`immich-go` is more than just an uploader; it intelligently interacts with the Immich server to preserve your library's structure:

- **Albums and Tags**: Automatically creates albums and tags on the server to match your source organization.
- **Stacking**: Groups related images, like RAW+JPEG pairs or photo bursts, into stacks.
- **Duplicate Detection**: Avoids re-uploading files that already exist on the server.
- **Efficient Uploads**: Can pause Immich's background jobs (like thumbnailing) during an upload for better performance.

For a detailed explanation of how each upload command works, please see the [Upload Commands Overview](docs/upload-commands-overview.md).

## 🎯 Popular Use Cases

- **Google Photos Migration**: [Complete guide](docs/best-practices.md#google-photos-migration)
- **iCloud Import**: [Step-by-step instructions](docs/examples.md#icloud-import)
- **Server Migration**: [Transfer between Immich instances](docs/examples.md#server-migration)
- **Bulk Organization**: [Stacking and tagging strategies](docs/best-practices.md#organization-strategies)

## 🤝 Contributing

Contributions are welcome! Please see our [contributing guidelines](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

---

**Need help?** Check our [documentation](docs/) or open an issue on GitHub.
