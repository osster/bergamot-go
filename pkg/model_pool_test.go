package bergamot

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTranslatorsShareAndSerializeModelUntilLastClose(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	bridge := &pooledTestBridge{}
	var initializeCalls atomic.Int32
	initialize := func(string, int) (translationBridge, error) {
		initializeCalls.Add(1)
		return bridge, nil
	}
	first, err := newTranslator(configPath, initialize)
	if err != nil {
		t.Fatalf("create first translator: %v", err)
	}
	second, err := newTranslator(configPath, initialize)
	if err != nil {
		t.Fatalf("create second translator: %v", err)
	}

	var workers sync.WaitGroup
	for _, translator := range []*Translator{first, second} {
		workers.Add(1)
		go func(translator *Translator) {
			defer workers.Done()
			if _, err := translator.Translate("Hello", "en-de"); err != nil {
				t.Errorf("Translate() error = %v", err)
			}
		}(translator)
	}
	workers.Wait()

	if got := initializeCalls.Load(); got != 1 {
		t.Errorf("model initializations = %d, want 1", got)
	}
	if got := bridge.maxConcurrent.Load(); got != 1 {
		t.Errorf("concurrent calls into shared model = %d, want 1", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first translator: %v", err)
	}
	if got := bridge.closeCalls.Load(); got != 0 {
		t.Errorf("model close calls with one live owner = %d, want 0", got)
	}
	if _, err := second.Translate("Still loaded", "en-de"); err != nil {
		t.Fatalf("translate after first owner closed: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second translator: %v", err)
	}
	if got := bridge.closeCalls.Load(); got != 1 {
		t.Errorf("model close calls after final owner closed = %d, want 1", got)
	}
}

type pooledTestBridge struct {
	active        atomic.Int32
	maxConcurrent atomic.Int32
	closeCalls    atomic.Int32
}

func (b *pooledTestBridge) Translate(input string) (string, error) {
	active := b.active.Add(1)
	for current := b.maxConcurrent.Load(); active > current; current = b.maxConcurrent.Load() {
		if b.maxConcurrent.CompareAndSwap(current, active) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	b.active.Add(-1)
	return "translated: " + input, nil
}

func (b *pooledTestBridge) Close() error {
	b.closeCalls.Add(1)
	return nil
}
