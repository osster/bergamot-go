package bergamot

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidConfig           = errors.New("invalid translator config")
	ErrUnsupportedLanguagePair = errors.New("unsupported language pair")
	ErrModelInitialization     = errors.New("model initialization failed")
	ErrTranslation             = errors.New("translation failed")
	ErrTranslatorClosed        = errors.New("translator is closed")
	ErrEmptyInput              = errors.New("translation input is empty")
	ErrResourceCleanup         = errors.New("resource cleanup failed")
	ErrInvalidChunkSize        = errors.New("stream chunk size must be positive")
	ErrStreamRead              = errors.New("stream read failed")
	ErrOperationCanceled       = errors.New("translation operation canceled")
	ErrOperationTimeout        = errors.New("translation operation timed out")
)

// Error describes a translator operation failure. Kind can be checked with
// errors.Is, while Unwrap exposes the underlying parsing, filesystem, or native
// bridge error when one exists.
type Error struct {
	Op           string
	LanguagePair string
	Kind         error
	Err          error
}

func (e *Error) Error() string {
	message := e.Op
	if e.LanguagePair != "" {
		message += " for " + e.LanguagePair
	}
	if e.Kind != nil {
		if message != "" {
			message += ": "
		}
		message += e.Kind.Error()
	}
	if e.Err != nil {
		if message != "" {
			message += ": "
		}
		message += e.Err.Error()
	}
	return message
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Is(target error) bool { return target == e.Kind }

func newError(kind error, op, languagePair string, err error) *Error {
	return &Error{Op: op, LanguagePair: languagePair, Kind: kind, Err: err}
}

// Config lists the model configuration files available for translation.
// Language-pair keys use the form "source-target", such as "en-de".
type Config struct {
	Defaults      TranslationOptions            `yaml:"defaults" json:"defaults"`
	LanguagePairs map[string]LanguagePairConfig `yaml:"language_pairs" json:"language_pairs"`
}

// TranslationOptions supplies optional defaults for all language pairs.
// Zero values leave the corresponding engine option unchanged or disabled.
type TranslationOptions struct {
	BeamSize      int    `yaml:"beam_size" json:"beam_size"`
	Timeout       string `yaml:"timeout" json:"timeout"`
	ContextWindow int    `yaml:"context_window" json:"context_window"`
}

// LanguagePairConfig configures the Bergamot model used for a language pair.
type LanguagePairConfig struct {
	ModelConfig   string  `yaml:"model_config" json:"model_config"`
	BeamSize      *int    `yaml:"beam_size,omitempty" json:"beam_size,omitempty"`
	Timeout       *string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ContextWindow *int    `yaml:"context_window,omitempty" json:"context_window,omitempty"`
}

// LoadConfig parses and validates a YAML or JSON translator configuration.
// Relative model_config paths remain relative here; NewTranslator resolves
// them against the directory containing this configuration file.
func LoadConfig(path string) (*Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, newError(ErrInvalidConfig, "read translator config", "", err)
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return nil, newError(ErrInvalidConfig, "parse translator config", "", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple YAML documents are not supported")
		}
		return nil, newError(ErrInvalidConfig, "parse translator config", "", err)
	}
	if len(config.LanguagePairs) == 0 {
		return nil, newError(ErrInvalidConfig, "validate translator config", "", errors.New("language_pairs must contain at least one pair"))
	}
	if err := validateTranslationOptions(config.Defaults); err != nil {
		return nil, newError(ErrInvalidConfig, "validate translator config defaults", "", err)
	}

	normalizedPairs := make(map[string]LanguagePairConfig, len(config.LanguagePairs))
	for configuredPair, pairConfig := range config.LanguagePairs {
		pair := normalizeLanguagePair(configuredPair)
		if !validLanguagePair(pair) {
			return nil, newError(ErrInvalidConfig, "validate translator config", configuredPair, errors.New("language pair must use source-target format"))
		}
		if _, exists := normalizedPairs[pair]; exists {
			return nil, newError(ErrInvalidConfig, "validate translator config", configuredPair, errors.New("duplicate language pair after normalization"))
		}
		if strings.TrimSpace(pairConfig.ModelConfig) == "" {
			return nil, newError(ErrInvalidConfig, "validate translator config", pair, errors.New("model_config must not be empty"))
		}
		pairConfig.ModelConfig = strings.TrimSpace(pairConfig.ModelConfig)
		options := config.Defaults
		if pairConfig.BeamSize != nil {
			options.BeamSize = *pairConfig.BeamSize
		}
		if pairConfig.Timeout != nil {
			options.Timeout = *pairConfig.Timeout
		}
		if pairConfig.ContextWindow != nil {
			options.ContextWindow = *pairConfig.ContextWindow
		}
		if err := validateTranslationOptions(options); err != nil {
			return nil, newError(ErrInvalidConfig, "validate translator config", pair, err)
		}
		beamSize, timeout, contextWindow := options.BeamSize, options.Timeout, options.ContextWindow
		pairConfig.BeamSize = &beamSize
		pairConfig.Timeout = &timeout
		pairConfig.ContextWindow = &contextWindow
		normalizedPairs[pair] = pairConfig
	}
	config.LanguagePairs = normalizedPairs
	return &config, nil
}

func validateTranslationOptions(options TranslationOptions) error {
	if options.BeamSize < 0 || options.BeamSize > 256 {
		return errors.New("beam_size must be 0 (unset) or between 1 and 256")
	}
	if options.ContextWindow < 0 || options.ContextWindow > 100 {
		return errors.New("context_window must be between 0 and 100 sentences")
	}
	if options.Timeout != "" {
		timeout, err := time.ParseDuration(options.Timeout)
		if err != nil || timeout <= 0 || timeout > 24*time.Hour {
			return errors.New("timeout must be a duration greater than 0 and at most 24h")
		}
	}
	return nil
}

func normalizeLanguagePair(languagePair string) string {
	return strings.ToLower(strings.TrimSpace(languagePair))
}

func validLanguagePair(languagePair string) bool {
	parts := strings.Split(languagePair, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, part := range parts {
		for _, char := range part {
			if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				return false
			}
		}
	}
	return true
}

func resolveModelConfig(configPath, modelConfig string) (string, error) {
	if !filepath.IsAbs(modelConfig) {
		modelConfig = filepath.Join(filepath.Dir(configPath), modelConfig)
	}
	resolved, err := filepath.Abs(modelConfig)
	if err != nil {
		return "", fmt.Errorf("resolve model config path %q: %w", modelConfig, err)
	}
	return resolved, nil
}
