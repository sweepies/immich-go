// Package stack provides CLI flag registration and configuration mapping
// for the stack command. It separates CLI concerns from the stack execution.
package stack

import (
	"time"

	"github.com/sweepies/immich-go/internal/filters"
	"github.com/spf13/pflag"
)

// Config holds all configuration for a stack operation.
type Config struct {
	// Server configuration
	Server ServerConfig

	// Stack options
	StackOptions StackOptions

	// Date range
	DateAfter  string
	DateBefore string
}

// ServerConfig holds Immich server connection settings.
type ServerConfig struct {
	Server        string
	APIKey        string
	AdminAPIKey   string
	APITrace      bool
	SkipSSL       bool
	ClientTimeout time.Duration
	DeviceUUID    string
	TimeZone      string
	DryRun        bool
}

// StackOptions holds configuration for photo stacking behavior.
type StackOptions struct {
	ManageHEICJPG       filters.HeicJpgFlag
	ManageRawJPG        filters.RawJPGFlag
	ManageBurst         filters.BurstFlag
	ManageEpsonFastFoto bool
}

// Flags holds all CLI flags for the stack command.
type Flags struct {
	// Server connection flags
	Server        string
	APIKey        string
	AdminAPIKey   string
	APITrace      bool
	SkipSSL       bool
	ClientTimeout time.Duration
	DeviceUUID    string
	TimeZone      string
	DryRun        bool

	// Stack options
	ManageHEICJPG       string
	ManageRawJPG        string
	ManageBurst         string
	ManageEpsonFastFoto bool

	// Date range flags
	DateAfter  string
	DateBefore string
}

// NewFlags creates a new Flags with default values.
func NewFlags() *Flags {
	return &Flags{
		ClientTimeout: 20 * time.Minute,
	}
}

// RegisterFlags adds all stack-related flags to the flag set.
func (f *Flags) RegisterFlags(flags *pflag.FlagSet) {
	// Server connection flags
	flags.StringVarP(&f.Server, "server", "s", f.Server, "Immich server address")
	flags.StringVarP(&f.APIKey, "api-key", "k", "", "API Key")
	flags.StringVar(&f.AdminAPIKey, "admin-api-key", "", "Admin's API Key")
	flags.BoolVar(&f.APITrace, "api-trace", false, "Enable trace of api calls")
	flags.BoolVar(&f.SkipSSL, "skip-verify-ssl", false, "Skip SSL verification")
	flags.DurationVar(&f.ClientTimeout, "client-timeout", f.ClientTimeout, "Set server calls timeout")
	flags.StringVar(&f.DeviceUUID, "device-uuid", f.DeviceUUID, "Set a device UUID")
	flags.StringVar(&f.TimeZone, "time-zone", f.TimeZone, "Override the system time zone")
	flags.BoolVar(&f.DryRun, "dry-run", false, "Simulate all actions")

	// Stack options
	flags.StringVar(&f.ManageHEICJPG, "manage-heic-jpeg", "", "Manage coupled HEIC and JPEG files")
	flags.StringVar(&f.ManageRawJPG, "manage-raw-jpeg", "", "Manage coupled RAW and JPEG files")
	flags.StringVar(&f.ManageBurst, "manage-burst", "", "Manage burst photos")
	flags.BoolVar(&f.ManageEpsonFastFoto, "manage-epson-fastfoto", false, "Manage Epson FastFoto file")

	// Date range flags
	flags.StringVar(&f.DateAfter, "date-after", "", "Only process files after this date (YYYY-MM-DD)")
	flags.StringVar(&f.DateBefore, "date-before", "", "Only process files before this date (YYYY-MM-DD)")
}

// Validate checks flags for errors before building configuration.
func (f *Flags) Validate() error {
	// Stack command validation is minimal - server connection is required
	// but that's handled by the client.Open call
	return nil
}

// ToConfig converts CLI flags to a stack configuration.
func (f *Flags) ToConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Server:        f.Server,
			APIKey:        f.APIKey,
			AdminAPIKey:   f.AdminAPIKey,
			APITrace:      f.APITrace,
			SkipSSL:       f.SkipSSL,
			ClientTimeout: f.ClientTimeout,
			DeviceUUID:    f.DeviceUUID,
			TimeZone:      f.TimeZone,
			DryRun:        f.DryRun,
		},
		StackOptions: f.buildStackOptions(),
		DateAfter:    f.DateAfter,
		DateBefore:   f.DateBefore,
	}

	return cfg, nil
}

func (f *Flags) buildStackOptions() StackOptions {
	opts := StackOptions{
		ManageEpsonFastFoto: f.ManageEpsonFastFoto,
	}

	if f.ManageHEICJPG != "" {
		_ = opts.ManageHEICJPG.Set(f.ManageHEICJPG)
	}
	if f.ManageRawJPG != "" {
		_ = opts.ManageRawJPG.Set(f.ManageRawJPG)
	}
	if f.ManageBurst != "" {
		_ = opts.ManageBurst.Set(f.ManageBurst)
	}

	return opts
}
