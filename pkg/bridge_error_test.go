package bergamot

import (
	"path/filepath"
	"strings"
	"testing"
)

// Init must turn a native model-config failure into a useful Go error.
func TestInitReturnsErrorForMissingModelConfig(t *testing.T) {
	missingConfig := filepath.Join(t.TempDir(), "missing-model.yml")
	bridge, err := Init(missingConfig)
	if err == nil {
		if bridge != nil {
			_ = bridge.Close()
		}
		t.Fatal("Init with a missing model config succeeded; want an error")
	}
	if bridge != nil {
		_ = bridge.Close()
		t.Fatal("Init returned a bridge together with an error")
	}
	if !strings.Contains(err.Error(), "unable to open model config") {
		t.Fatalf("Init error = %q, want native model-config failure", err)
	}
}

// Translation failures from the C ABI become useful Go errors and never
// expose a result. A null native handle reaches the wrapper's explicit failure
// path without requiring model artifacts.
func TestTranslateReturnsErrorForNativeFailure(t *testing.T) {
	result, err := translateWithNullHandle()
	if err == nil {
		t.Fatal("translation with a null native handle succeeded; want an error")
	}
	if result != "" {
		t.Fatalf("translation result on failure = %q, want empty result", result)
	}
	if !strings.Contains(err.Error(), "Bergamot handle is null") {
		t.Fatalf("translation error = %q, want native null-handle failure", err)
	}
}

// Close is idempotent, and calls after cleanup report an ordinary Go error
// instead of passing an invalid native handle to C++.
func TestBridgeLifecycleAfterClose(t *testing.T) {
	bridge := &Bridge{}
	if err := bridge.Close(); err != nil {
		t.Fatalf("close empty bridge: %v", err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("close bridge a second time: %v", err)
	}
	if _, err := bridge.Translate("hello"); err == nil {
		t.Fatal("Translate on a closed bridge succeeded; want an error")
	}
	if err := (*Bridge)(nil).Close(); err != nil {
		t.Fatalf("close nil bridge: %v", err)
	}
}
