// Package archive provides CLI command building for the archive command.
package archive

import (
	"context"

	"github.com/spf13/cobra"
)

// CommandBuilder builds the archive command with proper flag registration.
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

// Build creates a cobra command with all archive flags registered.
func (b *CommandBuilder) Build(ctx context.Context, runFunc func(cmd *cobra.Command, args []string, flags *Flags) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive [flags] <paths>...",
		Short: "Archive various sources of photos to a file system",
		Long: `Archive photos from various sources to a local file system.

By default, archives from local folders. Use source flags to change the source:
  --google      Import from Google Photos takeout
  --icloud      Import from iCloud takeout  
  --picasa      Enable Picasa album parsing
  --from-immich Transfer from another Immich server (no paths required)`,
		Args: func(cmd *cobra.Command, args []string) error {
			return nil // Validate in PreRunE
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return b.flags.Validate(args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFunc(cmd, args, b.flags)
		},
	}

	// Register all archive flags
	b.flags.RegisterFlags(cmd.Flags())

	// Mark required flags
	_ = cmd.MarkFlagRequired("write-to-folder")

	return cmd
}
