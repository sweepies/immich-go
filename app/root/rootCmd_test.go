package root

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		outputChan <- buf.String()
	}()

	err = fn()

	_ = writer.Close()
	output := <-outputChan
	_ = reader.Close()
	return output, err
}

func TestRootCommandRejectsInvalidOutput(t *testing.T) {
	ctx := context.Background()
	cmd, _ := RootImmichGoCommand(ctx)
	cmd.SetArgs([]string{"--output=bad", "version"})

	err := cmd.ExecuteContext(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

func TestRootCommandAutoDetectsNonInteractive(t *testing.T) {
	ctx := context.Background()
	cmd, app := RootImmichGoCommand(ctx)

	output, err := captureStdout(t, func() error {
		cmd.SetArgs([]string{"version"})
		return cmd.ExecuteContext(ctx)
	})

	require.NoError(t, err)
	assert.True(t, app.NonInteractive)
	assert.True(t, strings.Contains(output, "immich-go version"))
}
