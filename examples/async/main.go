package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	bergamot "github.com/osster/bergamot-go/pkg"
)

type request struct {
	text   string
	result <-chan bergamot.TranslationResult
}

func main() {
	configPath := flag.String("config", "examples/config-file/translator.yml", "translator YAML config path")
	pair := flag.String("pair", "en-de", "language pair to translate")
	flag.Parse()

	translator, err := bergamot.NewTranslator(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer translator.Close()

	ctx := context.Background()
	texts := []string{"Hello, world.", "Good morning.", "Thank you for your help."}
	requests := make([]request, 0, len(texts))
	for _, text := range texts {
		requests = append(requests, request{
			text:   text,
			result: translator.TranslateAsync(ctx, text, *pair),
		})
	}

	var failures int
	for _, req := range requests {
		result, ok := <-req.result
		if !ok {
			fmt.Fprintf(os.Stderr, "%q: async result channel closed without a result\n", req.text)
			failures++
			continue
		}
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "%q: %v\n", req.text, result.Err)
			failures++
			continue
		}
		fmt.Printf("%s\t%s\n", req.text, result.Text)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	canceledResult := <-translator.TranslateAsync(canceledCtx, "This request is canceled.", *pair)
	if !errors.Is(canceledResult.Err, bergamot.ErrOperationCanceled) {
		fmt.Fprintf(os.Stderr, "canceled request error = %v, want %v\n", canceledResult.Err, bergamot.ErrOperationCanceled)
		failures++
	} else {
		fmt.Printf("canceled request before translation started: %v\n", canceledResult.Err)
	}

	if failures > 0 {
		os.Exit(1)
	}
}
