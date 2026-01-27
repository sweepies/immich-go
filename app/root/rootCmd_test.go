package root

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommandRejectsInvalidOutput(t *testing.T) {
	ctx := context.Background()
	cmd, _ := RootImmichGoCommand(ctx)
	cmd.SetArgs([]string{"--output=bad", "version"})

	err := cmd.ExecuteContext(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}
