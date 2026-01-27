# AGENTS.md

## Project Overview

This project, `immich-go`, is a command-line tool written in Go. Its primary purpose is to provide an efficient way to upload large photo and video collections to a self-hosted [Immich](https://immich.app/) server.

It is designed as a cross-platform application (Windows, macOS, Linux, FreeBSD) with no external dependencies like Node.js or Docker, making it easy to install and use.

Key features include:
-   Uploading from multiple sources: local folders, Google Photos Takeout archives, iCloud, and even other Immich servers.
-   Smart management of assets, including duplicate detection, photo stacking (bursts, RAW+JPEG), and metadata handling.
-   A rich set of commands for interacting with Immich, such as `upload`, `archive`, and `stack`.

The project uses the Cobra library for its CLI structure and `goreleaser` to manage its release process.

## Release Notes

When asked to generate release notes, follow this format and guidance:

- Start with a brief, friendly introduction.
- Use these sections in order:
  - ✨ New Features (user-visible functionality)
  - 🚀 Improvements (enhancements to existing features)
  - 🐛 Bug Fixes (fixes to existing functionality)
  - 💥 Breaking Changes (changes requiring user action)
  - 🔧 Internal Changes (refactoring, CI/CD, tests - only if significant)
- Remove technical prefixes like feat:, fix:, chore:, refactor:, doc:, e2e:, test:.
- Write from the user's perspective and combine related commits.
- Explain CLI changes, list affected flags, and add examples if needed.
- Skip trivial changes (typos, README tweaks) and purely internal changes unless they impact users.

## Fork Release Versioning

- For this fork, bump the upstream minor version and append the suffix `-sweepy` to the release tag/version.
- Example: upstream v0.31.0 -> fork release v0.32.0-sweepy.
- Apply the `-sweepy` suffix to all future fork releases and use that version when generating release notes and running Goreleaser.

## Building and Running

### Prerequisites

-   Go (version 1.25 or higher, as specified in `go.mod`)

### Building the Application

To build the application from the source, run the following command from the project root:

```sh
go build -o immich-go main.go
```

For a production-ready, statically linked binary similar to the CI build, use:

```sh
CGO_ENABLED=0 go build -ldflags="-s -w -extldflags=-static" -o immich-go main.go
```

### Running the Application

Once built, the application can be run directly from the command line:

```sh
./immich-go --help
```

The main commands are `upload`, `archive`, and `stack`. Here is a basic usage example:

```sh
./immich-go upload --server http://your-immich-server:2283 --api-key YOUR_API_KEY /path/to/photos
```

## Development

### Testing

The project has unit tests.

**Unit Tests:**

Run all unit tests with race condition detection and coverage:

```sh
go test -race -v -count=1 -coverprofile=coverage.out ./...
```

E2E infrastructure was removed during simplification, so only unit tests are available right now.

# Version Control

Use semantic/conventional commit messages
