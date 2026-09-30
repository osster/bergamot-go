package bergamot

import (
	"path/filepath"
	"testing"
)

// Init must return a Go error for a missing model config without invoking the
// native parser with an empty configuration.
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
}
