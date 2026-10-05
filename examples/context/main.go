package main

import (
	"flag"
	"fmt"
	"os"

	bergamot "github.com/osster/bergamot-go/pkg"
)

type document struct {
	name      string
	sentences []string
}

func main() {
	configPath := flag.String("config", "examples/context/translator.yml", "translator YAML config path")
	pair := flag.String("pair", "en-de", "language pair to translate")
	flag.Parse()

	translator, err := bergamot.NewTranslator(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer translator.Close()

	documents := []document{
		{name: "first document", sentences: []string{"The train is late.", "It will arrive in ten minutes."}},
		{name: "unrelated document", sentences: []string{"The garden is quiet.", "A bird is sitting on the fence."}},
	}
	for _, doc := range documents {
		if err := translator.ResetContext(*pair); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s:\n", doc.name)
		for _, sentence := range doc.sentences {
			translation, err := translator.Translate(sentence, *pair)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("%s\t%s\n", sentence, translation)
		}
	}
}
