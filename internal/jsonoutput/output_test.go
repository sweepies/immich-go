package jsonoutput

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/simulot/immich-go/internal/assettracker"
	"github.com/simulot/immich-go/internal/fileevent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStdout(t *testing.T, fn func()) string {
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

	fn()

	_ = writer.Close()
	output := <-outputChan
	_ = reader.Close()
	return output
}

func TestWriteProgressOutputsJSONLine(t *testing.T) {
	output := captureStdout(t, func() {
		err := WriteProgress(42, 120, 3, 10)
		require.NoError(t, err)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 1)

	var progress ProgressUpdate
	err := json.Unmarshal([]byte(lines[0]), &progress)
	require.NoError(t, err)

	assert.Equal(t, "progress", progress.Type)
	assert.Equal(t, 42, progress.ImmichReadPct)
	assert.Equal(t, int64(120), progress.AssetsFound)
	assert.Equal(t, int64(3), progress.UploadErrors)
	assert.Equal(t, int64(10), progress.Uploaded)
	assert.False(t, progress.Timestamp.IsZero())
}

func TestWriteSummaryOutputsJSONLine(t *testing.T) {
	counters := assettracker.AssetCounters{
		Pending:       1,
		Processed:     2,
		Discarded:     3,
		Errors:        4,
		AssetSize:     100,
		ProcessedSize: 50,
	}
	countKey := fileevent.ErrorServerError
	successKey := fileevent.ProcessedUploadSuccess
	eventCounts := map[fileevent.Code]int64{
		countKey:   2,
		successKey: 5,
	}
	eventSizes := map[fileevent.Code]int64{
		countKey: 2048,
	}

	output := captureStdout(t, func() {
		err := WriteSummary("error", 1, counters, eventCounts, eventSizes, 12.5)
		require.NoError(t, err)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 1)

	var summary FinalSummary
	err := json.Unmarshal([]byte(lines[0]), &summary)
	require.NoError(t, err)

	assert.Equal(t, "summary", summary.Type)
	assert.Equal(t, "error", summary.Status)
	assert.Equal(t, 1, summary.ExitCode)
	assert.Equal(t, counters, summary.Counters)
	assert.Equal(t, 12.5, summary.DurationSeconds)
	assert.False(t, summary.Timestamp.IsZero())

	assert.Len(t, summary.Events, 2)
	assert.Equal(t, int64(2), summary.Events[countKey.String()].Count)
	assert.Equal(t, int64(2048), summary.Events[countKey.String()].Size)
	assert.Equal(t, int64(5), summary.Events[successKey.String()].Count)
	assert.Equal(t, int64(0), summary.Events[successKey.String()].Size)
}
