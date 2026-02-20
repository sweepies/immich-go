# immich-go-improved: fast, multi-source uploader for Immich

<img src="https://raw.githubusercontent.com/sweepies/immich-go/refs/heads/main/logo.jpg" alt="logo" width="512"/>

**immich-go-improved** is an open-source fork designed to streamline uploading large photo collections to your Immich server while still shipping the `immich-go` binary.

> ℹ️ **This is a fork** of [simulot/immich-go](https://github.com/simulot/immich-go). See [Fork Differences](#-fork-differences) for details about this fork.

> ⚠️ This is an early version, not yet extensively tested<br>
> ⚠️ Keep a backup copy of your files for safety<br>

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/sweepies/immich-go)

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
- **Simplified CLI** – Flag-based source selection (e.g., `upload --google`)

### Improvements

- **Performance enhancements** – Multiple algorithmic optimizations:
  - Google Photos metadata matching: O(n²) → O(n) for large takeout imports
  - Case-insensitive sidecar file lookup using efficient map-based indexing
  - String builder optimization for CSV report generation
- **Unix-style output** – Consistent stdout/stderr separation: reports to stdout, logs and progress to stderr
- **Output validation** – Invalid `--output` values now show clear validation errors
- **Streamlined interface** – Removed TUI, config files, and simplified command structure

### Bug Fixes

- **Duplicate summaries** – Prevented double JSON summaries for upload subcommands
- **Duplicate reports** – Removed duplicate final report output in text mode
- **Error reporting** – JSON summaries now reflect upload failures and write errors

### Breaking Changes

- **CLI restructured** – Source selection now uses flags (`--google`, `--icloud`, `--picasa`, `--from-immich`)
- **Config files removed** – The `--config` and `--save-config` flags have been removed; use environment variables or CLI flags
- **TUI removed** – Interactive TUI has been removed; output is always text or JSON
- **Logging flags removed** – The `--log-file` and `--log-type` flags have been removed; use shell redirection instead (e.g., `2>logs.txt`)

### Release Versioning

- Fork releases use a `-improved` version suffix (e.g., `v0.32.0-improved`)
- See the [releases page](https://github.com/sweepies/immich-go/releases) for version history

For the upstream project, see [simulot/immich-go](https://github.com/simulot/immich-go).

## 🚀 Quick Start

### 1. Install immich-go

Download the pre-built binary for your system from the [GitHub releases page](https://github.com/sweepies/immich-go/releases).

### 2. Basic Usage

```bash
# Upload photos from a local folder (default mode)
immich-go upload --server=http://your-ip:2283 --api-key=your-api-key /path/to/your/photos

# Upload Google Photos takeout
immich-go upload --google --server=http://your-ip:2283 --api-key=your-api-key /path/to/takeout-*.zip

# Archive photos from Immich server
immich-go archive --from-immich --write-to-folder=/path/to/archive --from-server=http://your-ip:2283 --from-api-key=your-api-key

# JSON output for automation
immich-go upload --server=http://your-ip:2283 --api-key=your-api-key --output=json /path/to/photos
```

### 3. Requirements

- A running Immich server with API access
- API key with appropriate permissions ([see full list](https://sweepies.github.io/immich-go/guide/installation#api-permissions))
- Go 1.26 or newer if you build from source

## 🙈 Skip System Files

- Use `--ban-file` to exclude junk artifacts. Patterns ending with `/` apply to directories (for example, `--ban-file .Spotlight-V100/`), while patterns without the trailing slash apply to individual files (for example, `--ban-file .DS_Store`).
- immich-go ships with sensible defaults that already skip common clutter such as `@eaDir/`, `@__thumb/`, `SYNOFILE_THUMB_*.*`, `Lightroom Catalog/`, `thumbnails/`, `.DS_Store`, `/._*`, `.Spotlight-V100/`, `.photostructure/`, and `Recently Deleted/`.
- Add additional patterns as needed to keep uploads focused on real photos. See the [banned files reference](https://sweepies.github.io/immich-go/commands/upload#banned-files) for details.

## 📚 Documentation

| Topic | Description |
|-------|-------------|
| [Installation](https://sweepies.github.io/immich-go/guide/installation) | Detailed installation instructions for all platforms |
| [Commands](https://sweepies.github.io/immich-go/commands/upload) | Complete command reference and options |
| [Configuration](https://sweepies.github.io/immich-go/reference/configuration) | Configuration options and environment variables |
| [Examples](https://sweepies.github.io/immich-go/guide/getting-started) | Common use cases and practical examples |
| [Best Practices](https://sweepies.github.io/immich-go/guide/best-practices) | Tips for optimal performance and reliability |
| [Technical Details](https://sweepies.github.io/immich-go/commands/upload#how-it-works) | File processing, metadata handling, and advanced features |
| [Upload Commands Overview](https://sweepies.github.io/immich-go/commands/upload#specialized-modes) | How `immich-go` processes files from different sources |
| [Contributor Guide](CONTRIBUTING.md) | Development setup, Go version baseline, and release note workflow |
| [Release Notes](https://sweepies.github.io/immich-go/releases/) | Version history and release notes |

## ✨ How immich-go Works

`immich-go` offers a versatile set of commands to handle your photo and video uploads. Whether you're uploading from a simple folder, migrating from a Google Photos Takeout, or transferring assets between Immich servers, the tool provides intelligent features to preserve your metadata and organization.

Here's a brief overview of the main upload commands:

- **Default (folder)**: Upload from any local folder. Creates albums from directory structure and reads XMP sidecar files.
- **`--google`**: Migrate from a Google Photos Takeout. Intelligently matches photos with JSON metadata to preserve albums, descriptions, and locations.
- **`--from-immich`**: Server-to-server migration tool to copy assets between Immich instances with fine-grained filtering.
- **`--picasa`**: Enable Picasa album parsing to read `.picasa.ini` files and restore album organization.
- **`--icloud`**: Handle iCloud Photos takeout, correctly identifying creation dates and album structures from CSV files.

### Leveraging Immich's Features

`immich-go` is more than just an uploader; it intelligently interacts with the Immich server to preserve your library's structure:

- **Albums and Tags**: Automatically creates albums and tags on the server to match your source organization.
- **Stacking**: Groups related images, like RAW+JPEG pairs or photo bursts, into stacks.
- **Duplicate Detection**: Avoids re-uploading files that already exist on the server.
- **Efficient Uploads**: Can pause Immich's background jobs (like thumbnailing) during an upload for better performance.

For a detailed explanation of how each upload command works, please see the [Upload Commands Overview](https://sweepies.github.io/immich-go/commands/upload#specialized-modes).

## 🎯 Popular Use Cases

- **Google Photos Migration**: [Complete guide](https://sweepies.github.io/immich-go/guide/best-practices#google-photos-migration)
- **iCloud Import**: [Step-by-step instructions](https://sweepies.github.io/immich-go/guide/icloud#basic-import)
- **Server Migration**: [Transfer between Immich instances](https://sweepies.github.io/immich-go/guide/server-migration#basic-migration)
- **Bulk Organization**: [Stacking and tagging strategies](https://sweepies.github.io/immich-go/guide/best-practices#organization-strategies)

## 🏗️ Architecture

The codebase follows a modular architecture with clear separation of concerns:

```
cmd/immich-go          Main entry point
app/                   Command implementations (Cobra CLI)
  ├── root/            Root command and flag handling
  ├── upload/          Upload command (legacy, migrating to internal/upload)
  ├── archive/         Archive command
  └── stack/           Stack command
internal/              Core business logic
  ├── adapters/        Source interface definitions
  ├── appcontext/      Application context and dependency injection
  ├── assets/          Asset domain model and caching
  ├── cli/             CLI flag handling and configuration
  ├── journal/       Event logging and tracking
  ├── fileprocessor/   File processing coordination
  ├── immich/          Immich API client (domain-specific interfaces)
  ├── upload/          Upload pipeline and orchestration
  │   ├── pipeline/    Staged upload pipeline with testable stages
  │   └── source/      Source factory and adapters
  └── observability/   Reporting and logging utilities
adapters/              Source adapters (folder, Google Photos, iCloud, etc.)
immich/                Legacy Immich API client (being migrated)
```

Key design principles:
- **Staged pipeline**: Upload orchestration split into discovery, upload, and finalize stages
- **Narrow interfaces**: Domain-specific client interfaces instead of monolithic structs
- **Testable stages**: Each pipeline stage can be tested independently with mock clients
- **Explicit dependencies**: Configuration passed explicitly rather than via service locators

## 🤝 Contributing

Contributions are welcome! Please see our [contributing guidelines](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

---

**Need help?** Check our [documentation](https://sweepies.github.io/immich-go/) or open an issue on GitHub.
