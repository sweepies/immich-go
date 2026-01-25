package appcontext

import (
	"log/slog"
	"os"
	"testing"
	"time"

	cliflags "github.com/simulot/immich-go/internal/cliFlags"
)

func TestNew_Defaults(t *testing.T) {
	ctx := New()

	if ctx.DryRun() {
		t.Error("expected DryRun to be false by default")
	}
	if ctx.ConcurrentTasks() != 1 {
		t.Errorf("expected ConcurrentTasks to be 1, got %d", ctx.ConcurrentTasks())
	}
	if ctx.Output() != "text" {
		t.Errorf("expected Output to be 'text', got %q", ctx.Output())
	}
	if ctx.IsJSONOutput() {
		t.Error("expected IsJSONOutput to be false for text output")
	}
	if ctx.TimeZone() != time.Local {
		t.Errorf("expected TimeZone to be Local, got %v", ctx.TimeZone())
	}
	if ctx.SupportedMedia() == nil {
		t.Error("expected SupportedMedia to not be nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	ctx := New(
		WithDryRun(true),
		WithConcurrentTasks(4),
		WithOutput("json"),
		WithTimeZone(loc),
		WithLogger(logger),
	)

	if !ctx.DryRun() {
		t.Error("expected DryRun to be true")
	}
	if ctx.ConcurrentTasks() != 4 {
		t.Errorf("expected ConcurrentTasks to be 4, got %d", ctx.ConcurrentTasks())
	}
	if ctx.Output() != "json" {
		t.Errorf("expected Output to be 'json', got %q", ctx.Output())
	}
	if !ctx.IsJSONOutput() {
		t.Error("expected IsJSONOutput to be true for json output")
	}
	if ctx.TimeZone() != loc {
		t.Errorf("expected TimeZone to be %v, got %v", loc, ctx.TimeZone())
	}
	if ctx.Logger() != logger {
		t.Error("expected Logger to match")
	}
}

func TestContext_Derive(t *testing.T) {
	original := New(
		WithDryRun(false),
		WithConcurrentTasks(2),
		WithOutput("text"),
	)

	derived := original.Derive(
		WithDryRun(true),
		WithOutput("json"),
	)

	// Original should be unchanged
	if original.DryRun() {
		t.Error("original DryRun should still be false")
	}
	if original.Output() != "text" {
		t.Errorf("original Output should still be 'text', got %q", original.Output())
	}

	// Derived should have new values
	if !derived.DryRun() {
		t.Error("derived DryRun should be true")
	}
	if derived.Output() != "json" {
		t.Errorf("derived Output should be 'json', got %q", derived.Output())
	}
	// But preserve non-overridden values
	if derived.ConcurrentTasks() != 2 {
		t.Errorf("derived ConcurrentTasks should be 2, got %d", derived.ConcurrentTasks())
	}
}

func TestWithOnErrors(t *testing.T) {
	var onErrors cliflags.OnErrorsFlag
	_ = onErrors.Set("5") // Accept up to 5 errors

	ctx := New(WithOnErrors(onErrors))

	if ctx.OnErrors() != onErrors {
		t.Errorf("expected OnErrors to be %v, got %v", onErrors, ctx.OnErrors())
	}
}

func TestTimeZone_NilFallback(t *testing.T) {
	ctx := &Context{tz: nil}

	if ctx.TimeZone() != time.Local {
		t.Errorf("expected nil tz to fallback to Local, got %v", ctx.TimeZone())
	}
}
