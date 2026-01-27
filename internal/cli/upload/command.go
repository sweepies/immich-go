// Package upload provides CLI command building for the upload command.
package upload

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

// CommandBuilder builds the upload command with proper flag registration.
// It separates CLI concerns from the upload pipeline execution.
type CommandBuilder struct {
	flags *Flags
}

// NewCommandBuilder creates a new command builder with default flags.
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{
		flags: NewFlags(),
	}
}

// Flags returns the flags struct for inspection or testing.
func (b *CommandBuilder) Flags() *Flags {
	return b.flags
}

// Build creates a cobra command with all upload flags registered.
// The runFunc is called with the validated configuration.
func (b *CommandBuilder) Build(ctx context.Context, runFunc func(cmd *cobra.Command, args []string, flags *Flags) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload [flags] <paths>...",
		Short: "Upload photos to an Immich server from various sources",
		Long: `Upload photos to an Immich server from various sources.

By default, uploads from local folders. Use source flags to change the source:
  --google      Import from Google Photos takeout
  --icloud      Import from iCloud takeout  
  --picasa      Enable Picasa album parsing
  --from-immich Transfer from another Immich server (no paths required)`,
		Args: func(cmd *cobra.Command, args []string) error {
			// --from-immich doesn't require paths, others do
			if b.flags.FromImmich {
				if len(args) > 0 {
					return nil // Allow args for now, validate in PreRunE
				}
				return nil
			}
			return nil // Validate in PreRunE
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return b.flags.Validate(args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFunc(cmd, args, b.flags)
		},
	}

	// Register all upload flags
	b.flags.RegisterFlags(cmd.Flags())

	return cmd
}

// ParseTimeZone parses the timezone string from flags.
func ParseTimeZone(tz string) (*time.Location, error) {
	if tz == "" {
		return time.Local, nil
	}
	return time.LoadLocation(tz)
}
