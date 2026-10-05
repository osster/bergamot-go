package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	bergamot "github.com/osster/bergamot-go/pkg"
)

type message struct {
	pair string
	text string
}

type response struct {
	message
	translation string
	err         error
}

func main() {
	configPath := flag.String("config", "examples/worker-pool/translator.yml", "translator YAML config path")
	workerCount := flag.Int("workers", 4, "number of translation workers")
	queueSize := flag.Int("queue", 32, "maximum queued messages")
	flag.Parse()

	if *workerCount < 1 || *queueSize < 1 {
		fmt.Fprintln(os.Stderr, "workers and queue must both be greater than zero")
		os.Exit(2)
	}
	if err := run(*configPath, *workerCount, *queueSize); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(configPath string, workerCount, queueSize int) error {
	translators := make([]*bergamot.Translator, 0, workerCount)
	for i := 0; i < workerCount; i++ {
		translator, err := bergamot.NewTranslator(configPath)
		if err != nil {
			for _, opened := range translators {
				_ = opened.Close()
			}
			return fmt.Errorf("create translator for worker %d: %w", i, err)
		}
		translators = append(translators, translator)
	}
	defer func() {
		for _, translator := range translators {
			_ = translator.Close()
		}
	}()

	jobs := make(chan message, queueSize)
	results := make(chan response, workerCount)
	var workers sync.WaitGroup
	for _, translator := range translators {
		workers.Add(1)
		go func(translator *bergamot.Translator) {
			defer workers.Done()
			for job := range jobs {
				translated, err := translator.Translate(job.text, job.pair)
				results <- response{message: job, translation: translated, err: err}
			}
		}(translator)
	}

	go func() {
		workers.Wait()
		close(results)
	}()

	var output sync.WaitGroup
	var failures int
	output.Add(1)
	go func() {
		defer output.Done()
		for result := range results {
			if result.err != nil {
				fmt.Fprintf(os.Stderr, "%s\t%q: %v\n", result.pair, result.text, result.err)
				failures++
				continue
			}
			fmt.Printf("%s\t%s\n", result.pair, result.translation)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var inputErr error
	for scanner.Scan() {
		pair, text, ok := strings.Cut(scanner.Text(), "\t")
		if !ok || strings.TrimSpace(pair) == "" || strings.TrimSpace(text) == "" {
			inputErr = errors.New("each input line must be language-pair<TAB>text")
			break
		}
		jobs <- message{pair: pair, text: text}
	}
	if err := scanner.Err(); err != nil {
		inputErr = errors.Join(inputErr, fmt.Errorf("read messages: %w", err))
	}
	close(jobs)
	output.Wait()
	if inputErr != nil {
		return inputErr
	}
	if failures > 0 {
		return fmt.Errorf("%d translation(s) failed", failures)
	}
	return nil
}
