package bergamot_test

import (
	"fmt"
	"os"
	"path/filepath"

	bergamot "github.com/osster/bergamot-go/pkg"
)

func ExampleLoadConfig() {
	configPath, cleanup, err := writeExampleConfig()
	if err != nil {
		panic(err)
	}
	defer cleanup()

	config, err := bergamot.LoadConfig(configPath)
	if err != nil {
		panic(err)
	}
	fmt.Println(config.LanguagePairs["en-de"].ModelConfig)

	// Output:
	// models/en-de.yml
}

func ExampleNewTranslator() {
	configPath, cleanup, err := writeExampleConfig()
	if err != nil {
		panic(err)
	}
	defer cleanup()

	translator, err := bergamot.NewTranslator(configPath)
	if err != nil {
		panic(err)
	}
	defer translator.Close()
	fmt.Println("translator configured; model loads on first translation")

	// Output:
	// translator configured; model loads on first translation
}

func writeExampleConfig() (string, func(), error) {
	directory, err := os.MkdirTemp("", "bergamot-go-example-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(directory, "translator.yml")
	contents := []byte("language_pairs:\n  en-de:\n    model_config: models/en-de.yml\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		_ = os.RemoveAll(directory)
		return "", nil, err
	}
	return path, func() { _ = os.RemoveAll(directory) }, nil
}
