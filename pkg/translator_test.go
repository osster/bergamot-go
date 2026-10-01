package bergamot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestTranslatorTranslatesConfiguredLanguagePair(t *testing.T) {
	configPath := writeTranslatorConfig(t, "language_pairs:\n  en-de:\n    model_config: models/en-de.yml\n")
	var initializedPath string
	initializeCalls := 0
	bridge := &fakeTranslationBridge{translation: "Guten Tag"}
	translator, err := newTranslator(configPath, func(path string) (translationBridge, error) {
		initializeCalls++
		initializedPath = path
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
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
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

	translator, err = newTranslator(configPath, func(string) (translationBridge, error) {
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
	translator, err := newTranslator(configPath, func(string) (translationBridge, error) {
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

type fakeTranslationBridge struct {
	translation    string
	translateErr   error
	translateCalls int
	closeCalls     int
	closeErr       error
}

func (b *fakeTranslationBridge) Translate(string) (string, error) {
	b.translateCalls++
	return b.translation, b.translateErr
}

func (b *fakeTranslationBridge) Close() error {
	b.closeCalls++
	return b.closeErr
}

func writeTranslatorConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "translator.yml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
