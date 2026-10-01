package bergamot

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type translationBridge interface {
	Translate(string) (string, error)
	Close() error
}

type bridgeInitializer func(string) (translationBridge, error)

// Translator provides translation for configured language pairs. Models are
// initialized the first time their pair is used and reused until Close.
type Translator struct {
	mu           sync.Mutex
	modelConfigs map[string]string
	bridges      map[string]translationBridge
	initialize   bridgeInitializer
	closed       bool
}

// NewTranslator parses a language-pair configuration file. Each configured
// model is loaded on its first translation request, not during construction.
func NewTranslator(configPath string) (*Translator, error) {
	return newTranslator(configPath, func(path string) (translationBridge, error) {
		return Init(path)
	})
}

func newTranslator(configPath string, initialize bridgeInitializer) (*Translator, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return nil, newError(ErrInvalidConfig, "resolve translator config path", "", err)
	}
	if initialize == nil {
		return nil, newError(ErrInvalidConfig, "create translator", "", errors.New("model initializer must not be nil"))
	}

	modelConfigs := make(map[string]string, len(config.LanguagePairs))
	for languagePair, pairConfig := range config.LanguagePairs {
		modelPath, err := resolveModelConfig(configPath, pairConfig.ModelConfig)
		if err != nil {
			return nil, newError(ErrInvalidConfig, "resolve model config", languagePair, err)
		}
		modelConfigs[languagePair] = modelPath
	}
	return &Translator{
		modelConfigs: modelConfigs,
		bridges:      make(map[string]translationBridge),
		initialize:   initialize,
	}, nil
}

// Translate synchronously translates text with the model configured for
// languagePair (for example, "en-de"). It returns an error for empty input,
// unsupported pairs, closed translators, or native translation failures.
func (t *Translator) Translate(text, languagePair string) (string, error) {
	if t == nil {
		return "", newError(ErrTranslatorClosed, "translate", languagePair, nil)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return "", newError(ErrTranslatorClosed, "translate", languagePair, nil)
	}
	pair := normalizeLanguagePair(languagePair)
	if strings.TrimSpace(text) == "" {
		return "", newError(ErrEmptyInput, "translate", pair, nil)
	}
	modelPath, supported := t.modelConfigs[pair]
	if !supported {
		return "", newError(ErrUnsupportedLanguagePair, "translate", pair, nil)
	}

	bridge, loaded := t.bridges[pair]
	if !loaded {
		var err error
		bridge, err = t.initialize(modelPath)
		if err != nil {
			return "", newError(ErrModelInitialization, "initialize model", pair, err)
		}
		if bridge == nil {
			return "", newError(ErrModelInitialization, "initialize model", pair, errors.New("initializer returned a nil bridge"))
		}
		t.bridges[pair] = bridge
	}

	translation, err := bridge.Translate(text)
	if err != nil {
		return "", newError(ErrTranslation, "translate", pair, err)
	}
	return translation, nil
}

// Close releases all models loaded by this Translator. It is safe to call
// repeatedly; all bridges are closed even if one cleanup reports an error.
func (t *Translator) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	var closeErrors []error
	for pair, bridge := range t.bridges {
		if err := bridge.Close(); err != nil {
			closeErrors = append(closeErrors, newError(ErrResourceCleanup, "close model", pair, err))
		}
	}
	t.bridges = nil
	if err := errors.Join(closeErrors...); err != nil {
		return fmt.Errorf("%w: %w", ErrResourceCleanup, err)
	}
	return nil
}
