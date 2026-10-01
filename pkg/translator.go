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
// unsupported pairs, closed translators, or native translation failures.
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
	if settings.timeout > 0 {
		type outcome struct {
			text string
			err  error
		}
		result := make(chan outcome, 1)
		go func() {
			translated, err := t.translateWithoutTimeout(text, pair)
			result <- outcome{text: translated, err: err}
		}()
		timer := time.NewTimer(settings.timeout)
		defer timer.Stop()
		select {
		case completed := <-result:
			return completed.text, completed.err
		case <-timer.C:
			select {
			case completed := <-result:
				return completed.text, completed.err
			default:
				return "", newError(ErrOperationTimeout, "translate", pair, context.DeadlineExceeded)
			}
		}
	}
	return t.translateWithoutTimeout(text, pair)
}

func (t *Translator) translateWithoutTimeout(text, pair string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return "", newError(ErrTranslatorClosed, "translate", pair, nil)
	}
	settings, supported := t.pairSettings[pair]
	if !supported {
		return "", newError(ErrUnsupportedLanguagePair, "translate", pair, nil)
	}

	bridge, loaded := t.bridges[pair]
	if !loaded {
		var err error
		bridge, err = acquireSharedBridge(modelPoolKey{modelConfig: settings.modelConfig, beamSize: settings.beamSize}, t.initialize)
		if err != nil {
			return "", newError(ErrModelInitialization, "initialize model", pair, err)
		}
		t.bridges[pair] = bridge
	}

	var translation string
	var err error
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
// its settings. Loaded models are retained unless their path or beam size changed.
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
	t.context = nil
	if err := errors.Join(closeErrors...); err != nil {
		return fmt.Errorf("%w: %w", ErrResourceCleanup, err)
	}
	return nil
}
