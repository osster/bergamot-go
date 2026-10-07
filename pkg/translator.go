package bergamot

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type translationBridge interface {
	Translate(string) (string, error)
	Close() error
}

type contextualTranslationBridge interface {
	TranslateWithContext(string, string) (string, error)
}

type batchTranslationBridge interface {
	TranslateMultiple([]string) ([]string, error)
}

// translateMultiple uses the bridge's batch call, or translates one input at a time
func translateMultiple(bridge translationBridge, inputs []string) ([]string, error) {
	if batch, ok := bridge.(batchTranslationBridge); ok {
		translations, err := batch.TranslateMultiple(inputs)
		if err == nil && len(translations) != len(inputs) {
			err = fmt.Errorf("got %d translations for %d inputs", len(translations), len(inputs))
		}
		return translations, err
	}
	translations := make([]string, len(inputs))
	for i, input := range inputs {
		translation, err := bridge.Translate(input)
		if err != nil {
			return nil, err
		}
		translations[i] = translation
	}
	return translations, nil
}

type bridgeInitializer func(string, int) (translationBridge, error)

type pairSettings struct {
	modelConfig   string
	beamSize      int
	timeout       time.Duration
	contextWindow int
}

// Translator provides translation for configured language pairs. Models are
// initialized the first time their pair is used and reused until Close.
type Translator struct {
	mu           sync.Mutex
	configPath   string
	pairSettings map[string]pairSettings
	context      map[string][]string
	bridges      map[string]translationBridge
	initialize   bridgeInitializer
	closed       bool
}

// NewTranslator parses a language-pair configuration file. Each configured
// model is loaded on its first translation request, not during construction.
func NewTranslator(configPath string) (*Translator, error) {
	return newTranslator(configPath, func(path string, beamSize int) (translationBridge, error) {
		return initWithBeamSize(path, beamSize)
	})
}

func newTranslator(configPath string, initialize bridgeInitializer) (*Translator, error) {
	settings, absoluteConfigPath, err := loadPairSettings(configPath)
	if err != nil {
		return nil, err
	}
	if initialize == nil {
		return nil, newError(ErrInvalidConfig, "create translator", "", errors.New("model initializer must not be nil"))
	}
	translator := &Translator{
		configPath:   absoluteConfigPath,
		pairSettings: settings,
		context:      make(map[string][]string),
		bridges:      make(map[string]translationBridge),
		initialize:   initialize,
	}
	runtime.SetFinalizer(translator, finalizeTranslator)
	return translator, nil
}

func finalizeTranslator(translator *Translator) {
	_ = translator.Close()
}

func loadPairSettings(configPath string) (map[string]pairSettings, string, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, "", err
	}
	absoluteConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, "", newError(ErrInvalidConfig, "resolve translator config path", "", err)
	}
	settings := make(map[string]pairSettings, len(config.LanguagePairs))
	for languagePair, pairConfig := range config.LanguagePairs {
		modelPath, err := resolveModelConfig(absoluteConfigPath, pairConfig.ModelConfig)
		if err != nil {
			return nil, "", newError(ErrInvalidConfig, "resolve model config", languagePair, err)
		}
		var timeout time.Duration
		if pairConfig.Timeout != nil && *pairConfig.Timeout != "" {
			timeout, _ = time.ParseDuration(*pairConfig.Timeout)
		}
		settings[languagePair] = pairSettings{
			modelConfig:   modelPath,
			beamSize:      *pairConfig.BeamSize,
			timeout:       timeout,
			contextWindow: *pairConfig.ContextWindow,
		}
	}
	return settings, absoluteConfigPath, nil
}

// Translate synchronously translates text with the model configured for
// languagePair (for example, "en-de"). It returns an error for empty input,
// unsupported pairs, closed translators, or native translation failures. A
// configured timeout bounds the caller's wait, but does not stop native inference.
func (t *Translator) Translate(text, languagePair string) (string, error) {
	if t == nil {
		return "", newError(ErrTranslatorClosed, "translate", languagePair, nil)
	}
	pair := normalizeLanguagePair(languagePair)
	if strings.TrimSpace(text) == "" {
		return "", newError(ErrEmptyInput, "translate", pair, nil)
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return "", newError(ErrTranslatorClosed, "translate", pair, nil)
	}
	settings, supported := t.pairSettings[pair]
	t.mu.Unlock()
	if !supported {
		return "", newError(ErrUnsupportedLanguagePair, "translate", pair, nil)
	}
	return withTimeout(settings.timeout, pair, func() (string, error) {
		return t.translateWithoutTimeout(text, pair)
	})
}

// TranslateMultiple translates the texts with the model configured for
// languagePair in one native call, so the engine packs their sentences into
// shared batches. Results are in input order. It fails like Translate, and for
// any empty text. It neither uses nor updates the context history. A configured
// timeout bounds the whole call.
func (t *Translator) TranslateMultiple(texts []string, languagePair string) ([]string, error) {
	if t == nil {
		return nil, newError(ErrTranslatorClosed, "translate", languagePair, nil)
	}
	pair := normalizeLanguagePair(languagePair)
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, newError(ErrEmptyInput, "translate", pair, nil)
		}
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, newError(ErrTranslatorClosed, "translate", pair, nil)
	}
	settings, supported := t.pairSettings[pair]
	t.mu.Unlock()
	if !supported {
		return nil, newError(ErrUnsupportedLanguagePair, "translate", pair, nil)
	}
	if len(texts) == 0 {
		return nil, nil
	}
	return withTimeout(settings.timeout, pair, func() ([]string, error) {
		t.mu.Lock()
		defer t.mu.Unlock()
		bridge, err := t.bridgeLocked(pair)
		if err != nil {
			return nil, err
		}
		translations, err := translateMultiple(bridge, texts)
		if err != nil {
			return nil, newError(ErrTranslation, "translate", pair, err)
		}
		return translations, nil
	})
}

// withTimeout runs translate, bounding the caller's wait when timeout is set;
// native inference is not stopped and finishes in the background
func withTimeout[T any](timeout time.Duration, pair string, translate func() (T, error)) (T, error) {
	if timeout <= 0 {
		return translate()
	}
	type outcome struct {
		value T
		err   error
	}
	result := make(chan outcome, 1)
	go func() {
		value, err := translate()
		result <- outcome{value: value, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case completed := <-result:
		return completed.value, completed.err
	case <-timer.C:
		select {
		case completed := <-result:
			return completed.value, completed.err
		default:
			var zero T
			return zero, newError(ErrOperationTimeout, "translate", pair, context.DeadlineExceeded)
		}
	}
}

// bridgeLocked returns the pair's bridge, initializing its model on first use.
// The pair must be configured; the caller holds t.mu.
func (t *Translator) bridgeLocked(pair string) (translationBridge, error) {
	if t.closed {
		return nil, newError(ErrTranslatorClosed, "translate", pair, nil)
	}
	settings, supported := t.pairSettings[pair]
	if !supported {
		return nil, newError(ErrUnsupportedLanguagePair, "translate", pair, nil)
	}
	if bridge, loaded := t.bridges[pair]; loaded {
		return bridge, nil
	}
	bridge, err := acquireSharedBridge(modelPoolKey{modelConfig: settings.modelConfig, beamSize: settings.beamSize}, t.initialize)
	if err != nil {
		return nil, newError(ErrModelInitialization, "initialize model", pair, err)
	}
	t.bridges[pair] = bridge
	return bridge, nil
}

func (t *Translator) translateWithoutTimeout(text, pair string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	bridge, err := t.bridgeLocked(pair)
	if err != nil {
		return "", err
	}
	settings := t.pairSettings[pair]

	var translation string
	if contextualBridge, ok := bridge.(contextualTranslationBridge); ok {
		translation, err = contextualBridge.TranslateWithContext(strings.Join(t.context[pair], " "), text)
	} else {
		translation, err = bridge.Translate(text)
	}
	if err != nil {
		return "", newError(ErrTranslation, "translate", pair, err)
	}
	if settings.contextWindow > 0 {
		history := append(t.context[pair], splitSentences(text)...)
		if len(history) > settings.contextWindow {
			history = history[len(history)-settings.contextWindow:]
		}
		t.context[pair] = history
	} else {
		delete(t.context, pair)
	}
	return translation, nil
}

// ResetContext clears the rolling source-sentence history for a language pair.
// It is useful when starting an unrelated document or conversation.
func (t *Translator) ResetContext(languagePair string) error {
	if t == nil {
		return newError(ErrTranslatorClosed, "reset context", languagePair, nil)
	}
	pair := normalizeLanguagePair(languagePair)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return newError(ErrTranslatorClosed, "reset context", pair, nil)
	}
	if _, supported := t.pairSettings[pair]; !supported {
		return newError(ErrUnsupportedLanguagePair, "reset context", pair, nil)
	}
	delete(t.context, pair)
	return nil
}

// ReloadConfig validates the current configuration file and atomically applies
// its settings. Loaded models are retained unless their path or beam size changed;
// removed or changed models are released, and their context history is cleared.
func (t *Translator) ReloadConfig() error {
	if t == nil {
		return newError(ErrTranslatorClosed, "reload translator config", "", nil)
	}
	settings, _, err := loadPairSettings(t.configPath)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return newError(ErrTranslatorClosed, "reload translator config", "", nil)
	}
	var closeErrors []error
	for pair, bridge := range t.bridges {
		oldSettings, oldExists := t.pairSettings[pair]
		newSettings, newExists := settings[pair]
		if oldExists && newExists && oldSettings.modelConfig == newSettings.modelConfig && oldSettings.beamSize == newSettings.beamSize {
			continue
		}
		if closeErr := bridge.Close(); closeErr != nil {
			closeErrors = append(closeErrors, newError(ErrResourceCleanup, "close reloaded model", pair, closeErr))
		}
		delete(t.bridges, pair)
		delete(t.context, pair)
	}
	for pair, history := range t.context {
		if newSettings, exists := settings[pair]; !exists || newSettings.contextWindow == 0 {
			delete(t.context, pair)
		} else if len(history) > newSettings.contextWindow {
			t.context[pair] = history[len(history)-newSettings.contextWindow:]
		}
	}
	t.pairSettings = settings
	if err := errors.Join(closeErrors...); err != nil {
		return fmt.Errorf("%w: %w", ErrResourceCleanup, err)
	}
	return nil
}

// Close releases this Translator's model references. Shared models remain loaded
// while another translator uses them. Close is safe to call repeatedly; all
// bridges are closed even if one cleanup reports an error.
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
	t.context = nil
	if err := errors.Join(closeErrors...); err != nil {
		return fmt.Errorf("%w: %w", ErrResourceCleanup, err)
	}
	return nil
}
