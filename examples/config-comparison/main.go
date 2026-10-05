package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	bergamot "github.com/osster/bergamot-go/pkg"
)

type message struct {
	pair string
	text string
}

type profileResult struct {
	translations      []string
	loadDuration      time.Duration
	translateDuration time.Duration
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (runErr error) {
	configA := flag.String("config-a", "examples/common/model-profiles/interactive-low-latency.yml", "first Bergamot model config path")
	configB := flag.String("config-b", "examples/common/model-profiles/quality-focused.yml", "second Bergamot model config path")
	text := flag.String("text", "Hello, world.", "text to translate with both configurations")
	stdinDataset := flag.Bool("stdin", false, "read pair<TAB>text messages from stdin and compare the full dataset")
	flag.Parse()

	messages := []message{{text: *text}}
	if *stdinDataset {
		var err error
		messages, err = readDataset(os.Stdin)
		if err != nil {
			return err
		}
	}

	resultA, err := translate(*configA, messages)
	if err != nil {
		return fmt.Errorf("config-a: %w", err)
	}

	resultB, err := translate(*configB, messages)
	if err != nil {
		return fmt.Errorf("config-b: %w", err)
	}

	if *stdinDataset {
		matches := 0
		for i := range resultA.translations {
			if resultA.translations[i] == resultB.translations[i] {
				matches++
			}
		}
		fmt.Printf("Compared %d messages (language pair %s)\n", len(messages), messages[0].pair)
		fmt.Printf("Matching outputs: %d/%d\n", matches, len(messages))
		printProfileMetrics("Config A", *configA, resultA, len(messages))
		printProfileMetrics("Config B", *configB, resultB, len(messages))
		return nil
	}

	fmt.Printf("Config A (%s): %s\n", *configA, resultA.translations[0])
	fmt.Printf("Config B (%s): %s\n", *configB, resultB.translations[0])
	fmt.Printf("Outputs match: %t\n", resultA.translations[0] == resultB.translations[0])
	return nil
}

func readDataset(reader io.Reader) ([]message, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var messages []message
	pair := ""
	for line := 1; scanner.Scan(); line++ {
		languagePair, text, ok := strings.Cut(scanner.Text(), "\t")
		languagePair = strings.TrimSpace(languagePair)
		text = strings.TrimSpace(text)
		if !ok || languagePair == "" || text == "" {
			return nil, fmt.Errorf("dataset line %d must contain language-pair<TAB>text", line)
		}
		if pair == "" {
			pair = languagePair
		} else if languagePair != pair {
			return nil, fmt.Errorf("dataset contains multiple language pairs (%q and %q)", pair, languagePair)
		}
		messages = append(messages, message{pair: languagePair, text: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	if len(messages) == 0 {
		return nil, errors.New("dataset is empty")
	}
	return messages, nil
}

func translate(configPath string, messages []message) (result profileResult, runErr error) {
	loadStart := time.Now()
	bridge, err := bergamot.Init(configPath)
	if err != nil {
		return profileResult{}, fmt.Errorf("initialize: %w", err)
	}
	result.loadDuration = time.Since(loadStart)
	defer func() {
		if closeErr := bridge.Close(); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close: %w", closeErr))
		}
	}()

	result.translations = make([]string, len(messages))
	translateStart := time.Now()
	for i, input := range messages {
		translation, err := bridge.Translate(input.text)
		if err != nil {
			return profileResult{}, fmt.Errorf("translate message %d: %w", i+1, err)
		}
		result.translations[i] = translation
	}
	result.translateDuration = time.Since(translateStart)
	return result, nil
}

func printProfileMetrics(label, configPath string, result profileResult, count int) {
	average := result.translateDuration / time.Duration(count)
	operationsPerSecond := float64(count) / result.translateDuration.Seconds()
	fmt.Printf("%s (%s): model load %s; translations %s total, %s average, %.2f ops/sec\n",
		label, configPath, result.loadDuration, result.translateDuration, average, operationsPerSecond)
}
