// Package upload provides upload orchestration for immich-go.
// This file contains the configuration types for the upload command,
// separating pure data from CLI concerns.
package upload

import (
	"time"

	"github.com/simulot/immich-go/internal/adapters"
	"github.com/simulot/immich-go/internal/filters"
)

// SourceMode indicates which adapter to use for reading assets.
type SourceMode = adapters.SourceMode

// Re-export source mode constants for convenience.
const (
	SourceModeFolder     = adapters.SourceModeFolder
	SourceModeICloud     = adapters.SourceModeICloud
	SourceModePicasa     = adapters.SourceModePicasa
	SourceModeGoogle     = adapters.SourceModeGoogle
	SourceModeFromImmich = adapters.SourceModeFromImmich
)

// Config holds all configuration for an upload operation.
// This is a pure data structure with no CLI dependencies.
// It is created by the CLI layer and passed to the upload pipeline.
type Config struct {
	// Server configuration
	Server ServerConfig

	// Upload behavior
	Overwrite  bool     // Always overwrite files on the server with local versions
	Tags       []string // Tags to add to uploaded assets
	SessionTag bool     // Tag uploaded photos with a session tag

	// Source selection (determines which adapter to use)
	SourceMode SourceMode

	// Adapter-specific configuration (one will be used based on SourceMode)
	FolderConfig     *adapters.FolderConfig
	GoogleConfig     *adapters.GoogleConfig
	FromImmichConfig *adapters.FromImmichConfig

	// Stacking options
	StackOptions StackOptions

	// Paths from command line arguments
	Paths []string
}

// ServerConfig holds Immich server connection settings.
type ServerConfig struct {
	Server        string        // Immich server address
	APIKey        string        // API Key for user operations
	AdminAPIKey   string        // API Key for admin operations
	APITrace      bool          // Enable API call traces
	SkipSSL       bool          // Skip SSL verification
	ClientTimeout time.Duration // Request timeout
	DeviceUUID    string        // Device UUID for uploads
	TimeZone      string        // Timezone override
	DryRun        bool          // Simulate all actions
	PauseJobs     bool          // Pause Immich background jobs during upload
	NoResumeJobs  bool          // Do not resume Immich background jobs after upload
}

// StackOptions holds configuration for photo stacking behavior.
type StackOptions struct {
	ManageHEICJPG       filters.HeicJpgFlag
	ManageRawJPG        filters.RawJPGFlag
	ManageBurst         filters.BurstFlag
	ManageEpsonFastFoto bool
}

// Filters returns the filter functions based on stack options.
func (so *StackOptions) Filters() []filters.Filter {
	return []filters.Filter{
		so.ManageBurst.GroupFilter(),
		so.ManageRawJPG.GroupFilter(),
		so.ManageHEICJPG.GroupFilter(),
	}
}

// Validate checks the configuration for errors.
// This is called by the CLI layer before starting the upload.
func (c *Config) Validate() error {
	// Validation logic is handled by the CLI layer
	// This method exists for future expansion
	return nil
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ClientTimeout: 20 * time.Minute,
			PauseJobs:     true,
		},
		FolderConfig:     &adapters.FolderConfig{},
		GoogleConfig:     &adapters.GoogleConfig{},
		FromImmichConfig: &adapters.FromImmichConfig{},
	}
}
