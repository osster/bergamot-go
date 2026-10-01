package main

import (
	"errors"
	"strings"
	"testing"
)

type testBridge struct {
	input        string
	translation  string
	translateErr error
	closed       bool
}

func (b *testBridge) Translate(input string) (string, error) {
	b.input = input
	return b.translation, b.translateErr
}

func (b *testBridge) Close() error {
	b.closed = true
	return nil
}

func TestRunTranslatesArgumentAndStandardInput(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		input string
	}{
		{name: "arguments", args: []string{"-config", "model.yml", "hello", "world"}, input: "hello world"},
		{name: "standard input", args: []string{"-config", "model.yml"}, stdin: "hello from stdin", input: "hello from stdin"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bridge := &testBridge{translation: "translated text"}
			var output strings.Builder
			initialize := func(config string) (translationBridge, error) {
				if config != "model.yml" {
					t.Fatalf("model config = %q, want model.yml", config)
				}
				return bridge, nil
			}

			err := run(test.args, strings.NewReader(test.stdin), &output, initialize)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if bridge.input != test.input {
				t.Errorf("translated input = %q, want %q", bridge.input, test.input)
			}
			if output.String() != "translated text\n" {
				t.Errorf("output = %q, want translated text and newline", output.String())
			}
			if !bridge.closed {
				t.Error("bridge was not closed")
			}
		})
	}
}

func TestRunRejectsMissingConfigAndEmptyInputBeforeInitialization(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing config", args: []string{"hello"}, want: "model config is required"},
		{name: "empty input", args: []string{"-config", "model.yml"}, want: "translation input is empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initialize := func(string) (translationBridge, error) {
				t.Fatal("model initialization must not run for invalid arguments")
				return nil, nil
			}
			err := run(test.args, strings.NewReader(" \n"), &strings.Builder{}, initialize)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run error = %v, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestRunClosesBridgeWhenTranslationFails(t *testing.T) {
	bridge := &testBridge{translateErr: errors.New("native translation failure")}
	initialize := func(string) (translationBridge, error) { return bridge, nil }
	err := run([]string{"-config", "model.yml", "hello"}, strings.NewReader(""), &strings.Builder{}, initialize)
	if err == nil || !strings.Contains(err.Error(), "native translation failure") {
		t.Fatalf("run error = %v, want native translation failure", err)
	}
	if !bridge.closed {
		t.Error("bridge was not closed after translation failure")
	}
}

func TestRunHelpPrintsUsageWithoutInitializingModel(t *testing.T) {
	var output strings.Builder
	initialize := func(string) (translationBridge, error) {
		t.Fatal("model initialization must not run for help")
		return nil, nil
	}
	if err := run([]string{"-h"}, strings.NewReader(""), &output, initialize); err != nil {
		t.Fatalf("run help: %v", err)
	}
	if got := strings.Count(output.String(), "Usage:"); got != 1 {
		t.Fatalf("usage count = %d, want 1; output: %q", got, output.String())
	}
	if !strings.Contains(output.String(), "-config string") {
		t.Fatalf("help output does not describe the config option: %q", output.String())
	}
}
