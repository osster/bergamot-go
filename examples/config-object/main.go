package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	bergamot "github.com/osster/bergamot-go/pkg"
	"gopkg.in/yaml.v3"
)

type translatorConfig struct {
	Defaults      translationOptions            `yaml:"defaults"`
	LanguagePairs map[string]languagePairConfig `yaml:"language_pairs"`
}

type translationOptions struct {
	BeamSize int `yaml:"beam_size,omitempty"`
}

type languagePairConfig struct {
	ModelConfig string `yaml:"model_config"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	modelConfig := flag.String("model-config", "pkg/testdata/bridge-smoke-mozilla-en-de.yml", "Bergamot model config path")
	text := flag.String("text", "Hello, world.", "text to translate")
	count := flag.Int("count", 1, "number of times to translate the text")
	flag.Parse()
	if *count < 1 {
		return errors.New("count must be greater than zero")
	}

	absoluteModelConfig, err := filepath.Abs(*modelConfig)
	if err != nil {
		return err
	}
	config := translatorConfig{
		Defaults: translationOptions{BeamSize: 1},
		LanguagePairs: map[string]languagePairConfig{
			"en-de": {ModelConfig: absoluteModelConfig},
		},
	}
	contents, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	configDirectory, err := os.MkdirTemp("", "bergamot-go-config-object-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDirectory)
	configPath := filepath.Join(configDirectory, "translator.yml")
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		return err
	}

	translator, err := bergamot.NewTranslator(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := translator.Close(); closeErr != nil {
			runErr = errors.Join(runErr, closeErr)
		}
	}()

	for i := 0; i < *count; i++ {
		translation, err := translator.Translate(*text, "en-de")
		if err != nil {
			return err
		}
		fmt.Println(translation)
	}
	return nil
}
