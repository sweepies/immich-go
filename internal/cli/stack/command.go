// Package stack provides CLI command building for the stack command.
package stack

import (
	"context"

	"github.com/spf13/cobra"
)

// CommandBuilder builds the stack command with proper flag registration.
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

// Build creates a cobra command with all stack flags registered.
func (b *CommandBuilder) Build(ctx context.Context, runFunc func(cmd *cobra.Command, args []string, flags *Flags) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack [flags]",
		Short: "Update Immich for stacking related photos",
		Long:  `Stack photos related to each other according to the options`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return b.flags.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFunc(cmd, args, b.flags)
		},
	}

	// Register all stack flags
	b.flags.RegisterFlags(cmd.Flags())

	return cmd
}
