package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"bergamot-go/pkg"
)

type translationBridge interface {
	Translate(string) (string, error)
	Close() error
}

type bridgeInitializer func(string) (translationBridge, error)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, initializeBridge); err != nil {
		fmt.Fprintln(os.Stderr, "bergamot-go:", err)
		os.Exit(1)
	}
}

func initializeBridge(configPath string) (translationBridge, error) {
	return bergamot.Init(configPath)
}

func run(args []string, stdin io.Reader, stdout io.Writer, initialize bridgeInitializer) error {
	flags := flag.NewFlagSet("bergamot-go", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {
		fmt.Fprintln(stdout, "Usage: bergamot-go -config MODEL.yml [TEXT ...]")
		fmt.Fprintln(stdout, "Translate TEXT, or read the input from stdin when TEXT is omitted.")
		fmt.Fprintln(stdout, "  -config string")
		fmt.Fprintln(stdout, "    \tpath to the Bergamot model configuration")
	}
	configPath := flags.String("config", "", "path to the Bergamot model configuration")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse arguments: %w (use -h for help)", err)
	}
	if *configPath == "" {
		return errors.New("model config is required (use -config MODEL.yml)")
	}

	input := strings.Join(flags.Args(), " ")
	if input == "" {
		contents, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read translation input: %w", err)
		}
		input = string(contents)
	}
	if strings.TrimSpace(input) == "" {
		return errors.New("translation input is empty; pass TEXT or pipe text to stdin")
	}

	bridge, err := initialize(*configPath)
	if err != nil {
		return fmt.Errorf("initialize model: %w", err)
	}
	if bridge == nil {
		return errors.New("initialize model: returned an empty bridge")
	}
	translation, translateErr := bridge.Translate(input)
	closeErr := bridge.Close()
	if translateErr != nil {
		return fmt.Errorf("translate input: %w", translateErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close model: %w", closeErr)
	}
	if _, err := fmt.Fprintln(stdout, translation); err != nil {
		return fmt.Errorf("write translation: %w", err)
	}
	return nil
}
