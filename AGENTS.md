# Project Overview

This project, `immich-go`, is a command-line tool written in Go. Its primary purpose is to provide an efficient way to upload large photo and video collections to a self-hosted [Immich](https://immich.app/) server.

It is designed as a cross-platform application (Windows, macOS, Linux, FreeBSD) with no external dependencies like Node.js or Docker, making it easy to install and use.

Key features include:
-   Uploading from multiple sources: local folders, Google Photos Takeout archives, iCloud, and even other Immich servers.
-   Smart management of assets, including duplicate detection, photo stacking (bursts, RAW+JPEG), and metadata handling.
-   A rich set of commands for interacting with Immich, such as `upload`, `archive`, and `stack`.

The project uses the Cobra library for its CLI structure and `goreleaser` to manage its release process.


# Changie

We use `changie` for changelog management. After making a **user-facing** change, you must document it via `changie new -b <body> -k <kind>`.

Use the conventional commit style for the body.

Valid kinds:
- Added
- Changed
- Deprecated
- Removed
- Fixed
- Security

This will generate a file under `.changes/unreleased/`. You can edit this file later if necessary.

# Testing

Run all unit tests with race condition detection and coverage:

```sh
go test -race -v -count=1 -coverprofile=coverage.out ./...
```

Use conventional commit messages.
