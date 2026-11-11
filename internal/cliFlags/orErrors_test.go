package cliflags

import (
	"strings"
	"testing"
)

// D4: a negative number is not a valid --on-errors value.
func TestOnErrorsFlagSetRejectsNegative(t *testing.T) {
	var f OnErrorsFlag
	err := f.Set("-5")
	if err == nil {
		t.Fatal("expected an error for a negative value")
	}
	if !strings.Contains(err.Error(), "on-errors") {
		t.Errorf("expected the flag name in the error message, got %q", err.Error())
	}
}

func TestOnErrorsFlagSetAcceptedValues(t *testing.T) {
	tests := []struct {
		value string
		want  OnErrorsFlag
		str   string
	}{
		{"stop", OnErrorsStop, "stop"},
		{"continue", OnErrorsNeverStop, "continue"},
		{"0", OnErrorsStop, "stop"},
		{"3", OnErrorsFlag(3), "3"},
	}
	for _, tt := range tests {
		var f OnErrorsFlag
		if err := f.Set(tt.value); err != nil {
			t.Errorf("Set(%q): unexpected error %v", tt.value, err)
			continue
		}
		if f != tt.want {
			t.Errorf("Set(%q) = %d, want %d", tt.value, f, tt.want)
		}
		if got := f.String(); got != tt.str {
			t.Errorf("String() after Set(%q) = %q, want %q", tt.value, got, tt.str)
		}
	}
}
