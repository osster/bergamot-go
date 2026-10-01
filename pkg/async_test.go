package bergamot

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTranslateAsyncRunsInBackgroundAndSerializesClose(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	started := make(chan struct{})
	release := make(chan struct{})
	bridge := &asyncTestBridge{started: started, release: release}
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		return bridge, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}

	resultChannel := translator.TranslateAsync(context.Background(), "Hello", "en-de")
	<-started
	select {
	case result := <-resultChannel:
		t.Fatalf("async result arrived before native translation completed: %+v", result)
	default:
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- translator.Close() }()
	close(release)
	result := <-resultChannel
	if result.Err != nil || result.Text != "translated: Hello" {
		t.Fatalf("async result = %+v, want successful translation", result)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := bridge.closeCalls.Load(); got != 1 {
		t.Errorf("bridge close calls = %d, want 1", got)
	}
}

func TestTranslateAsyncCancellationBeforeStart(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		t.Fatal("canceled async request initialized a model")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resultChannel := translator.TranslateAsync(ctx, "Hello", "en-de")
	result, ok := <-resultChannel
	if !ok || !errors.Is(result.Err, ErrOperationCanceled) || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("async result = %+v, want cancellation error", result)
	}
	if _, ok := <-resultChannel; ok {
		t.Fatal("async result channel was not closed")
	}
	_ = translator.Close()
}

func TestTranslateAsyncWithCallback(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		return &asyncTestBridge{}, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	callbackResults := make(chan TranslationResult, 1)
	results := translator.TranslateAsyncWithCallback(context.Background(), "Hello", "en-de", func(result TranslationResult) {
		callbackResults <- result
	})
	result := <-results
	callbackResult := <-callbackResults
	if result.Err != nil || result.Text != "translated: Hello" {
		t.Fatalf("async result = %+v, want successful translation", result)
	}
	if callbackResult != result {
		t.Errorf("callback result = %+v, channel result = %+v", callbackResult, result)
	}
	_ = translator.Close()
}

func TestTranslateStreamChunksAndReportsProgress(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	bridge := &asyncTestBridge{translate: func(text string) (string, error) { return text, nil }}
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		return bridge, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}

	const input = "éab 1234"
	var output strings.Builder
	wantBytes := []int64{4, 7, 9}
	index := 0
	for result := range translator.TranslateStream(context.Background(), strings.NewReader(input), "en-de", 3) {
		if result.Err != nil {
			t.Fatalf("TranslateStream() error = %v", result.Err)
		}
		output.WriteString(result.Text)
		if result.Progress.ChunkIndex != index+1 || result.Progress.ChunksCompleted != index+1 {
			t.Errorf("progress = %+v, want chunk index/completed %d", result.Progress, index+1)
		}
		if index >= len(wantBytes) || result.Progress.BytesProcessed != wantBytes[index] {
			t.Errorf("bytes processed = %d, want cumulative %v", result.Progress.BytesProcessed, wantBytes)
		}
		index++
	}
	if got := output.String(); got != input {
		t.Errorf("stream output = %q, want input %q", got, input)
	}
	if index != len(wantBytes) {
		t.Errorf("chunks emitted = %d, want %d", index, len(wantBytes))
	}
	if got := bridge.translateCalls.Load(); got != 3 {
		t.Errorf("native translation calls = %d, want 3", got)
	}
	if err := translator.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTranslateStreamReportsValidationAndReadErrors(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		return &asyncTestBridge{translate: func(text string) (string, error) { return text, nil }}, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	for result := range translator.TranslateStream(context.Background(), strings.NewReader("text"), "en-de", 0) {
		if !errors.Is(result.Err, ErrInvalidChunkSize) {
			t.Errorf("invalid chunk size error = %v, want ErrInvalidChunkSize", result.Err)
		}
	}

	readErr := errors.New("input failed")
	for result := range translator.TranslateStream(context.Background(), errorReader{err: readErr}, "en-de", 3) {
		if !errors.Is(result.Err, ErrStreamRead) || !errors.Is(result.Err, readErr) {
			t.Errorf("read error = %v, want wrapped stream read error", result.Err)
		}
	}
	for result := range translator.TranslateStream(context.Background(), strings.NewReader(""), "en-de", 3) {
		if !errors.Is(result.Err, ErrEmptyInput) {
			t.Errorf("empty stream error = %v, want ErrEmptyInput", result.Err)
		}
	}
	_ = translator.Close()
}

func TestTranslateStreamCancellation(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
		t.Fatal("canceled stream initialized a model")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for result := range translator.TranslateStream(ctx, strings.NewReader("text"), "en-de", 3) {
		if !errors.Is(result.Err, ErrOperationCanceled) || !errors.Is(result.Err, context.Canceled) {
			t.Errorf("stream cancellation error = %v, want context cancellation", result.Err)
		}
	}
	_ = translator.Close()
}

type asyncTestBridge struct {
	started        chan struct{}
	release        chan struct{}
	translate      func(string) (string, error)
	translateCalls atomic.Int32
	closeCalls     atomic.Int32
	startOnce      sync.Once
}

func (b *asyncTestBridge) Translate(text string) (string, error) {
	b.translateCalls.Add(1)
	if b.started != nil {
		b.startOnce.Do(func() { close(b.started) })
	}
	if b.release != nil {
		<-b.release
	}
	if b.translate != nil {
		return b.translate(text)
	}
	return "translated: " + text, nil
}

func (b *asyncTestBridge) Close() error {
	b.closeCalls.Add(1)
	return nil
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = errorReader{}
