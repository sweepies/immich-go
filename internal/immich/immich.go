// Package immich provides domain-specific interfaces and types for interacting
// with an Immich server. The package is organized around narrow interfaces
// that can be composed as needed by consumers.
//
// The main interfaces are:
//   - AssetsService: Asset operations (upload, download, update, delete)
//   - AlbumsService: Album management
//   - TagsService: Tag operations
//   - StacksService: Photo stacking
//   - JobsService: Server job control
//   - ServerService: Server-level operations (ping, stats, media types)
//
// Each interface is designed to be mockable for testing.
package immich

// Client combines all Immich service interfaces.
// This is the full interface for clients that need access to all operations.
type Client interface {
	AssetsService
	AlbumsService
	TagsService
	StacksService
	JobsService
	ServerService

	// UserID returns the current authenticated user's ID.
	UserID() UserID
}

// UploadClient defines the minimal interface needed for upload operations.
// This is a subset of Client focused on what the upload pipeline needs.
type UploadClient interface {
	AssetsService
	AlbumsService
	TagsService
	StacksService
	JobsService

	// UserID returns the current authenticated user's ID.
	UserID() UserID
}
