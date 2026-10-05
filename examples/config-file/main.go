package main

import (
	"flag"
	"fmt"
	"os"

	bergamot "github.com/osster/bergamot-go/pkg"
)

func main() {
	configPath := flag.String("config", "examples/config-file/translator.yml", "translator YAML config path")
	pair := flag.String("pair", "en-de", "language pair to translate")
	text := flag.String("text", "Hello, world.", "text to translate")
	count := flag.Int("count", 1, "number of times to translate the text")
	flag.Parse()
	if *count < 1 {
		fmt.Fprintln(os.Stderr, "count must be greater than zero")
		os.Exit(2)
	}

	translator, err := bergamot.NewTranslator(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer translator.Close()

	for i := 0; i < *count; i++ {
		translation, err := translator.Translate(*text, *pair)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(translation)
	}
}
