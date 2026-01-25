# immich-go Overhaul Plan

This document is a comprehensive, phased overhaul plan for the immich-go codebase. It focuses on separating responsibilities, improving testability, and making the architecture more idiomatic Go while preserving user-visible behavior.

## Goals
- Reduce the size and responsibility of the main upload orchestration to smaller, testable units.
- Replace the service-locator-style `app.Application` with explicit dependencies and narrow interfaces.
- Reshape the Immich API client into smaller, cohesive sub-clients with clearer ownership.
- Improve package layout so the codebase matches domain concepts (upload, assets, adapters, immich API).
- Increase test coverage and add deterministic pipeline tests.

## Non-goals
- No user-facing CLI flag removals in the initial migration phases.
- No behavior changes to upload semantics until parity tests exist.
- No full re-implementation of adapters unless required by the new pipeline.

## Current architecture hotspots
- `app/upload/run.go` contains orchestration, concurrency, reporting, server querying, and lifecycle logic in one file.
- `app/upload/upload.go` mixes flag parsing, adapter selection, lifecycle setup, and orchestration state in `UpCmd`.
- `app/app.go` behaves like a service locator with mutable global state across commands.
- `immich/immich.go` defines a large composite interface, and the client in `immich/*.go` mixes multiple domains.
- `internal/*` packages cover many domains without a clear layering or ownership model.

## Target architecture (overview)

```
cmd/immich-go
internal/cli          Cobra commands and flag parsing
internal/appcontext   App-wide config, logging, and DI wiring
internal/upload       Upload pipeline and orchestration
internal/adapters     Sources (folder, takeout, from-immich)
internal/assets       Asset domain model and indexing
internal/immich       API client split by domain
internal/observability Logging, file events, reporting
```

Key principles:
- Keep command setup in `internal/cli` and push all logic into domain packages.
- Prefer small interfaces at boundaries (ports) over global structs.
- Keep stateful orchestration in `internal/upload` and stateless helpers in `internal/observability`.

## Proposed package layout changes
- `app/*` moves to `internal/cli/*`, keeping Cobra-specific code out of core logic.
- `app/app.go` becomes `internal/appcontext/appcontext.go` with immutable configuration and explicit dependencies.
- `app/upload/run.go` and `app/upload/upload.go` become `internal/upload/*` with a pipeline API.
- `immich/*` becomes `internal/immich/*` with sub-clients for assets, albums, tags, stacks, jobs.
- `internal/fileevent`, `internal/fileprocessor`, and reporting helpers move under `internal/observability`.

## Phase 1: Dependency injection and app context ✅
Objective: replace the service-locator `app.Application` with explicit wiring.

1. Introduce `internal/appcontext.Context` containing immutable config, logging, and runtime flags.
2. Provide constructors for services currently created inside command handlers (e.g., file processor).
3. Replace direct `app.Application` usage in `app/upload/upload.go` with explicit dependencies.
4. Update command wiring in `app/root/rootCmd.go` to build the context once and pass it down.

Exit criteria:
- `app/app.go` no longer holds mutable global state or is only a thin wiring layer.
- New context is used by upload and archive commands without behavior changes.

## Phase 2: Upload pipeline extraction ✅
Objective: split upload orchestration into distinct stages with clear contracts.

1. Define pipeline stages in `internal/upload/pipeline`:
   - Source discovery
   - Metadata normalization
   - De-duplication and grouping
   - Upload and server sync
   - Finalize (albums, tags, summary)
2. Extract asset indexing logic from `app/upload/run.go` into `internal/assets/index`.
3. Move report generation and JSON output from `app/upload/run.go` into `internal/observability/reporting`.
4. Replace `UpCmd` state with a pipeline context struct owned by `internal/upload`.

Exit criteria:
- `app/upload/run.go` is mostly removed or becomes a small orchestration wrapper.
- Pipeline stages are independently testable with fake adapters and fake immich clients.

Implementation notes:
- Created `internal/upload/pipeline/` package with:
  - `pipeline.go`: Core pipeline orchestration with `Context`, `Stage`, and `Pipeline` types
  - `server.go`: `ServerClient` interface with narrow sub-interfaces (`AssetsClient`, `AlbumsClient`, etc.)
  - `index.go`: Asset index with de-duplication logic (extracted from `app/upload/advice.go`)
  - `stages.go`: Pipeline stages (`DiscoveryStage`, `AlbumDiscoveryStage`, `UploadStage`, `FinalizeStage`, `JobControlStage`)
  - `runner.go`: High-level runner that orchestrates the pipeline execution
  - `index_test.go`: Unit tests for the index and advice logic
- Created `internal/observability/reporting/reporting.go` for report generation
- Pipeline stages are independently testable with fake adapters and mock server clients

## Phase 3: Immich client redesign ✅
Objective: create cohesive sub-clients and simplify interfaces.

1. Split `immich/immich.go` into interfaces by domain in `internal/immich` (assets, albums, tags, stacks, jobs).
2. Encapsulate HTTP client creation in a `internal/immich/client` package with explicit options.
3. Replace global interface embedding with narrow interfaces passed into each upload stage.
4. Normalize error handling and add typed IDs for assets and albums where possible.

Exit criteria:
- Upload pipeline uses `AssetsClient`, `AlbumsClient`, `TagsClient`, etc. instead of `ImmichInterface`.
- HTTP client configuration is centralized and tested.

Implementation notes:
- Created `internal/immich/` package with domain-specific interfaces and types:
  - `types.go`: Typed IDs (`AssetID`, `AlbumID`, `TagID`, `StackID`, `UserID`) and time types
  - `assets.go`: `AssetsService` interface and `Asset`, `AssetResponse`, `AssetStatistics` types
  - `albums.go`: `AlbumsService` interface and `AlbumSimplified`, `AlbumContent` types
  - `tags.go`: `TagsService` interface and `TagSimplified`, `TagAssetsResponse` types
  - `stacks.go`: `StacksService` interface and `StackResponse` type
  - `jobs.go`: `JobsService` interface with `JobCommand`, `JobName` constants
  - `server.go`: `ServerService` interface and `User`, `ServerStatistics`, `AboutInfo` types
  - `immich.go`: Combined `Client` and `UploadClient` interfaces
- Created `internal/immich/client/` package for HTTP client configuration:
  - `client.go`: `Config` struct with functional options (`WithEndPoint`, `WithAPIKey`, etc.)
  - `client_test.go`: Unit tests for configuration
- Updated `internal/upload/pipeline/server.go`:
  - Pipeline interfaces now use typed IDs from `internal/immich`
  - `ServerClientAdapter` converts between old `immich` package and new `internal/immich` types
- Updated `internal/upload/pipeline/stages.go` and `index.go` to use new types
- All existing tests pass with updated types

## Phase 4: Adapter and source refactor ✅
Objective: make source adapters explicit, composable, and testable.

1. Define a `Source` interface in `internal/adapters` that returns a stream of `assets.Asset`.
2. Move adapter setup out of `app/upload/upload.go` into `internal/upload/source`.
3. Ensure `folder`, `googlePhotos`, `fromimmich` adapters implement a shared contract.
4. Provide adapters with only the dependencies they need (config, logger, asset index).

Exit criteria:
- `UpCmd` no longer owns adapter state.
- Each adapter has focused unit tests and can be swapped in pipeline tests.

Implementation notes:
- Created `internal/adapters/` package with:
  - `source.go`: `Source` interface with `Browse(ctx) <-chan *assets.Group` and `io.Closer`
  - `config.go`: Configuration types (`FolderConfig`, `GoogleConfig`, `FromImmichConfig`, `StackOptions`)
  - `SourceDependencies` struct with narrow dependencies (Logger, Processor, SupportedMedia, TimeZone, ConcurrentTasks)
  - `SourceMode` enum for source type identification
- Created `internal/upload/source/` package with:
  - `source.go`: `Factory` for creating sources from configuration
  - `folder.go`: `FolderSource` implementing `adapters.Source` for folder/iCloud/Picasa imports
  - `google.go`: `GoogleSource` stub (full migration in Phase 5/6)
  - `fromimmich.go`: `FromImmichSource` stub (full migration in Phase 5/6)
  - `adapter.go`: `LegacyReaderAdapter` for bridging old `Reader` to new `Source` interface
  - `source_test.go`: Unit tests for factory and adapter functionality
- Updated `adapters/adapters.go`: Added `Source` interface alongside legacy `Reader`
- Updated `internal/upload/pipeline/pipeline.go`: Uses `adapters.Source` type alias
- Legacy adapters remain functional; new `Source` interface enables gradual migration

## Phase 5: CLI refactor and command wiring ✅
Objective: reduce command types to configuration and routing only.

1. Replace `UpCmd` with a small config struct passed to the upload pipeline.
2. Move flag registration into `internal/cli/upload` and map flags to configuration.
3. Ensure consistent flag validation lives in CLI layer, not in pipeline stages.
4. Apply the same pattern to `archive` and `stack` commands as needed.

Exit criteria:
- Command types contain only flags and option mapping logic.
- Upload behavior remains identical from a user perspective.

Implementation notes:
- Created `internal/upload/config.go` with pure data `Config` struct (no CLI dependencies)
- Created `internal/cli/upload/` package with:
  - `flags.go`: `Flags` struct for CLI flag registration, validation, and config mapping
  - `command.go`: `CommandBuilder` for creating cobra commands with proper flag registration
- Created `internal/cli/archive/` package with:
  - `flags.go`: `Flags` struct and `Config` type for archive command
  - `command.go`: `CommandBuilder` for archive command
- Created `internal/cli/stack/` package with:
  - `flags.go`: `Flags` struct and `Config` type for stack command
  - `command.go`: `CommandBuilder` for stack command
- Refactored `app/upload/upload.go`:
  - Removed legacy `NewUploadCommand()` function
  - `NewUploadCommandFromCLI()` is now the only command creation method
  - `UpCmd` struct simplified to use `config` field for all configuration
  - Removed redundant flag fields (Overwrite, Tags, SessionTag, source mode flags)
- Refactored `app/archive/archiveCmd.go`:
  - Removed legacy `NewArchiveCommand()` function
  - `NewArchiveCommandFromCLI()` is now the only command creation method
  - `ArchiveCmd` struct simplified to use `config` field
- Refactored `app/stack/stack.go`:
  - Removed legacy `NewStackCommand()` function
  - `NewStackCommandFromCLI()` is now the only command creation method
  - `StackCmd` struct simplified to use `config` field
- Updated `app/root/rootCmd.go` to use new `*FromCLI` command functions
- All existing tests pass with the new structure

## Phase 6: Testing, docs, and cleanup ✅
Objective: raise confidence and remove unused legacy code.

1. Add unit tests for each pipeline stage.
2. Add integration tests for end-to-end upload using a mock immich client.
3. Remove legacy helpers in `app/upload` that are now replaced.
4. Update `README.md` and internal docs to match the new package layout.

Exit criteria:
- Pipeline stages each have dedicated test coverage.
- Upload flow is validated by at least one end-to-end test.

Implementation notes:
- Created `internal/upload/pipeline/stages_test.go` with comprehensive tests for:
  - `DiscoveryStage`: Tests for successful discovery, owner filtering, external library filtering, progress callbacks, error handling, context cancellation
  - `AlbumDiscoveryStage`: Tests for successful album discovery and error handling
  - `JobControlStage`: Tests for pause/resume jobs, error handling differences between pause and resume
  - `FinalizeStage`: Tests for cache closing behavior
  - `ParallelStage`: Tests for parallel execution and error propagation
  - `Pipeline.Run`: Tests for stage ordering and error stopping
- Created `internal/upload/pipeline/runner_test.go` with integration tests for:
  - End-to-end upload flow with mock server client
  - Job pausing/resuming during upload
  - Duplicate detection (same checksum, same name+date)
  - Album handling and saving
  - Context cancellation during upload
  - Error callback behavior
  - `NewContext` constructor tests
- Created `mockServerClient` implementing `ServerClient` interface for testing
- Created `mockSource` implementing `Source` interface for testing
- Added deprecation comment to `app/upload/advice.go` indicating migration to `internal/upload/pipeline`
- Updated `README.md` with new Architecture section documenting the package layout
- Legacy `app/upload/` code retained (still actively used) but marked for migration

## Migration strategy
- Maintain a compatibility layer in `internal/upload` that can accept legacy `UpCmd` inputs during transition.
- Introduce new stages incrementally while leaving legacy behavior in place behind a switch.
- Keep all CLI flags intact until parity tests validate the new pipeline.

## Risk register and mitigations
- Risk: behavior changes in upload decisions during refactor.
  Mitigation: add parity tests around `assetIndex.ShouldUpload` behavior in `app/upload`.
- Risk: regressions in adapter behavior (takeout, iCloud, from-immich).
  Mitigation: add adapter-specific tests using recorded fixtures and fake assets.
- Risk: increased API latency due to client redesign.
  Mitigation: add benchmarks for `GetAllAssets` and upload loops before refactoring.

## Validation plan
- Unit tests for each pipeline stage with fake adapters and fake clients.
- Integration tests for upload with a mock immich server client.
- Dry-run command tests validating summary output formatting.
- Performance baseline capturing total upload time for a fixed dataset.

## Suggested implementation order
1. Phase 0 and Phase 1 to stabilize dependencies and establish tests.
2. Phase 2 to carve out the pipeline (highest impact).
3. Phase 3 to split the client while pipeline is isolated.
4. Phase 4 and Phase 5 to clean up adapters and CLI.
5. Phase 6 for test expansion and cleanup.

## Key files and modules to review first
- `app/upload/run.go`
- `app/upload/upload.go`
- `app/app.go`
- `immich/immich.go`
- `immich/client.go`
- `internal/fileprocessor/*`
- `internal/assets/*`

## Done criteria
- Upload pipeline stages are independently testable and maintainable.
- `app.Application` is removed or reduced to wiring only.
- `UpCmd` becomes a configuration mapping only (no core business logic).
- Immich client is split into cohesive sub-clients with narrow interfaces.
- Test coverage increased for upload flow and adapters.
