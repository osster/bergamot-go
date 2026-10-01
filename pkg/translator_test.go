package bergamot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigSupportsYAMLAndJSON(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{
			name: "yaml",
			contents: "language_pairs:\n  en-de:\n    model_config: models/en-de.yml\n" +
				"  fr-en:\n    model_config: /models/fr-en.yml\n",
		},
		{
			name:     "json",
			contents: `{"language_pairs":{"en-de":{"model_config":"models/en-de.yml"}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "translator-config."+test.name)
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}

			config, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if got := config.LanguagePairs["en-de"].ModelConfig; got != "models/en-de.yml" {
				t.Errorf("model config = %q, want relative path unchanged", got)
			}
		})
	}
}

func TestLoadConfigRejectsMalformedEntries(t *testing.T) {
	for _, contents := range []string{
		"language_pairs: {}\n",
		"language_pairs:\n  ende:\n    model_config: model.yml\n",
		"language_pairs:\n  en-de: {}\n",
		"language_pairs:\n  en-de:\n    model_config: model.yml\n    unexpected: true\n",
		"language_pairs: [\n",
	} {
		t.Run(strings.ReplaceAll(contents, "\n", "/"), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "translator.yml")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("LoadConfig() error = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestLoadConfigAppliesDefaultsAndValidatesOverrides(t *testing.T) {
	configPath := writeTranslatorConfig(t, "defaults:\n  beam_size: 5\n  timeout: 2s\n  context_window: 3\nlanguage_pairs:\n  en-de:\n    model_config: model.yml\n  fr-en:\n    model_config: fr.yml\n    beam_size: 2\n    timeout: \"\"\n    context_window: 0\n")
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	defaultPair := config.LanguagePairs["en-de"]
	if *defaultPair.BeamSize != 5 || *defaultPair.Timeout != "2s" || *defaultPair.ContextWindow != 3 {
		t.Errorf("default options = beam %d, timeout %q, context %d", *defaultPair.BeamSize, *defaultPair.Timeout, *defaultPair.ContextWindow)
	}
	overridePair := config.LanguagePairs["fr-en"]
	if *overridePair.BeamSize != 2 || *overridePair.Timeout != "" || *overridePair.ContextWindow != 0 {
		t.Errorf("pair overrides = beam %d, timeout %q, context %d", *overridePair.BeamSize, *overridePair.Timeout, *overridePair.ContextWindow)
	}

	for _, option := range []string{"beam_size: 257", "timeout: 0s", "timeout: invalid", "context_window: -1", "context_window: 101"} {
		contents := "language_pairs:\n  en-de:\n    model_config: model.yml\n    " + option + "\n"
		if _, err := LoadConfig(writeTranslatorConfig(t, contents)); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("LoadConfig(%q) error = %v, want ErrInvalidConfig", option, err)
		}
	}
}

func TestTranslatorTranslatesConfiguredLanguagePair(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: models/en-de.yml\n")
	var initializedPath string
	initializeCalls := 0
	bridge := &fakeTranslationBridge{translation: "Guten Tag"}
	translator, err := newTranslator(configPath, func(path string, beamSize int) (translationBridge, error) {
		initializeCalls++
		initializedPath = path
		if beamSize != 0 {
			t.Errorf("beam size = %d, want default 0", beamSize)
		}
		return bridge, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	if initializeCalls != 0 {
		t.Fatalf("model initialization calls during construction = %d, want 0", initializeCalls)
	}

	for i := 0; i < 2; i++ {
		got, err := translator.Translate("Good day", "en-de")
		if err != nil {
			t.Fatalf("Translate() error = %v", err)
		}
		if got != "Guten Tag" {
			t.Errorf("Translate() = %q, want Guten Tag", got)
		}
	}
	wantPath := filepath.Join(filepath.Dir(configPath), "models", "en-de.yml")
	if initializedPath != wantPath {
		t.Errorf("initialized model config = %q, want %q", initializedPath, wantPath)
	}
	if initializeCalls != 1 {
		t.Errorf("model initialization calls = %d, want 1", initializeCalls)
	}
	if bridge.translateCalls != 2 {
		t.Errorf("native translation calls = %d, want 2", bridge.translateCalls)
	}
	if bridge.closeCalls != 0 {
		t.Errorf("native close calls before Close() = %d, want 0", bridge.closeCalls)
	}
	if err := translator.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := translator.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if bridge.closeCalls != 1 {
		t.Errorf("native close calls = %d, want 1", bridge.closeCalls)
	}
	if _, err := translator.Translate("Good day", "en-de"); !errors.Is(err, ErrTranslatorClosed) {
		t.Errorf("Translate() after Close() error = %v, want ErrTranslatorClosed", err)
	}
}

func TestTranslatorReportsUnsupportedPairAndBackendErrors(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	backendErr := errors.New("native failure")
	translator, err := newTranslator(configPath, func(string, int) (translationBridge, error) {
		return &fakeTranslationBridge{translateErr: backendErr}, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	if _, err := translator.Translate("Hello", "fr-en"); !errors.Is(err, ErrUnsupportedLanguagePair) {
		t.Errorf("unsupported pair error = %v, want ErrUnsupportedLanguagePair", err)
	}
	if _, err := translator.Translate("Hello", "en-de"); !errors.Is(err, ErrTranslation) || !errors.Is(err, backendErr) {
		t.Errorf("backend error = %v, want ErrTranslation wrapping native failure", err)
	}
	_ = translator.Close()

	translator, err = newTranslator(configPath, func(string, int) (translationBridge, error) {
		return nil, backendErr
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	if _, err := translator.Translate("Hello", "en-de"); !errors.Is(err, ErrModelInitialization) || !errors.Is(err, backendErr) {
		t.Errorf("initialization error = %v, want ErrModelInitialization wrapping native failure", err)
	}
	_ = translator.Close()
}

func TestTranslatorRejectsEmptyInput(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n")
	translator, err := newTranslator(configPath, func(string, int) (translationBridge, error) {
		t.Fatal("empty input initialized model")
		return nil, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	if _, err := translator.Translate(" \n", "en-de"); !errors.Is(err, ErrEmptyInput) {
		t.Errorf("empty input error = %v, want ErrEmptyInput", err)
	}
	_ = translator.Close()
}

func TestTranslatorMaintainsAndResetsContextWindow(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n    context_window: 2\n")
	bridge := &contextRecordingBridge{}
	translator, err := newTranslator(configPath, func(string, int) (translationBridge, error) { return bridge, nil })
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	defer translator.Close()

	for _, input := range []string{"First. Second.", "Third.", "Fourth."} {
		if _, err := translator.Translate(input, "en-de"); err != nil {
			t.Fatalf("Translate(%q) error = %v", input, err)
		}
	}
	wantContexts := []string{"", "First. Second.", "Second. Third."}
	if len(bridge.contexts) != len(wantContexts) {
		t.Fatalf("context calls = %d, want %d", len(bridge.contexts), len(wantContexts))
	}
	for i, want := range wantContexts {
		if got := bridge.contexts[i]; got != want {
			t.Errorf("context for call %d = %q, want %q", i+1, got, want)
		}
	}
	if err := translator.ResetContext(" EN-DE "); err != nil {
		t.Fatalf("ResetContext() error = %v", err)
	}
	if _, err := translator.Translate("New document.", "en-de"); err != nil {
		t.Fatalf("Translate() after ResetContext error = %v", err)
	}
	if got := bridge.contexts[len(bridge.contexts)-1]; got != "" {
		t.Errorf("context after reset = %q, want empty", got)
	}
}

func TestTranslatorReloadConfigReplacesOnlyChangedModels(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: old.yml\n")
	var bridges []*fakeTranslationBridge
	var initializedPath string
	var initializedBeam int
	translator, err := newTranslator(configPath, func(path string, beamSize int) (translationBridge, error) {
		initializedPath, initializedBeam = path, beamSize
		bridge := &fakeTranslationBridge{translation: "translated"}
		bridges = append(bridges, bridge)
		return bridge, nil
	})
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	defer translator.Close()
	if _, err := translator.Translate("Hello", "en-de"); err != nil {
		t.Fatalf("initial Translate() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("language_pairs:\n  fr-en:\n    model_config: new.yml\n    beam_size: 6\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := translator.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig() error = %v", err)
	}
	if bridges[0].closeCalls != 1 {
		t.Errorf("replaced bridge close calls = %d, want 1", bridges[0].closeCalls)
	}
	if _, err := translator.Translate("Hello", "en-de"); !errors.Is(err, ErrUnsupportedLanguagePair) {
		t.Errorf("Translate() for removed pair error = %v, want ErrUnsupportedLanguagePair", err)
	}
	if _, err := translator.Translate("Bonjour", "fr-en"); err != nil {
		t.Fatalf("Translate() for added pair error = %v", err)
	}
	if !strings.HasSuffix(initializedPath, "new.yml") || initializedBeam != 6 {
		t.Errorf("reloaded model init = path %q, beam %d", initializedPath, initializedBeam)
	}
	if err := os.WriteFile(configPath, []byte("language_pairs: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := translator.ReloadConfig(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("ReloadConfig() for invalid file error = %v, want ErrInvalidConfig", err)
	}
	if _, err := translator.Translate("Encore", "fr-en"); err != nil {
		t.Errorf("existing config after rejected reload returned error = %v", err)
	}
}

func TestTranslatorReturnsConfiguredTimeout(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: model.yml\n    timeout: 10ms\n")
	bridge := &fakeTranslationBridge{translate: func(string) (string, error) {
		time.Sleep(50 * time.Millisecond)
		return "late", nil
	}}
	translator, err := newTranslator(configPath, func(string, int) (translationBridge, error) { return bridge, nil })
	if err != nil {
		t.Fatalf("newTranslator() error = %v", err)
	}
	if _, err := translator.Translate("Hello", "en-de"); !errors.Is(err, ErrOperationTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Translate() timeout error = %v, want timeout and deadline errors", err)
	}
	if err := translator.Close(); err != nil {
		t.Fatalf("Close() after timeout = %v", err)
	}
}

type fakeTranslationBridge struct {
	translation    string
	translate      func(string) (string, error)
	translateErr   error
	translateCalls int
	closeCalls     int
	closeErr       error
}

func (b *fakeTranslationBridge) Translate(input string) (string, error) {
	b.translateCalls++
	if b.translate != nil {
		return b.translate(input)
	}
	return b.translation, b.translateErr
}

func (b *fakeTranslationBridge) Close() error {
	b.closeCalls++
	return b.closeErr
}

type contextRecordingBridge struct {
	contexts []string
}

func (b *contextRecordingBridge) Translate(input string) (string, error) { return input, nil }

func (b *contextRecordingBridge) TranslateWithContext(context, input string) (string, error) {
	b.contexts = append(b.contexts, context)
	return input, nil
}

func (b *contextRecordingBridge) Close() error { return nil }

func writeTranslatorConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "translator.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
