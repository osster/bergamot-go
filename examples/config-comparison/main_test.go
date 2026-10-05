package main

import (
	"strings"
	"testing"
)

func TestReadDataset(t *testing.T) {
	messages, err := readDataset(strings.NewReader("en-de\tHello.\nen-de\tGood morning.\n"))
	if err != nil {
		t.Fatalf("readDataset returned error: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("readDataset returned %d messages, want 2", len(messages))
	}
	if messages[0] != (message{pair: "en-de", text: "Hello."}) {
		t.Errorf("first message = %#v, want en-de Hello.", messages[0])
	}
	if messages[1] != (message{pair: "en-de", text: "Good morning."}) {
		t.Errorf("second message = %#v, want en-de Good morning.", messages[1])
	}
}

func TestReadDatasetRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "missing text", input: "en-de\t\n"},
		{name: "missing pair", input: "\tHello.\n"},
		{name: "missing separator", input: "Hello.\n"},
		{name: "multiple pairs", input: "en-de\tHello.\nfr-en\tBonjour.\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readDataset(strings.NewReader(test.input)); err == nil {
				t.Fatal("readDataset returned no error for invalid input")
			}
		})
	}
}
