package bergamot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBridgeSmoke exercises initialization, translation, and cleanup with the
// Mozilla EN-DE release. The model files must be downloaded separately.
func TestBridgeSmoke(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate bridge smoke test source")
	}
	projectRoot := filepath.Dir(filepath.Dir(sourceFile))

	configPath := os.Getenv("BERGAMOT_TEST_MODEL_CONFIG")
	if configPath == "" {
		modelPath := filepath.Join(projectRoot, "third_party", "bergamot-translator", "models", "mozilla-en-de", "model.ende.intgemm.alphas.bin")
		if _, err := os.Stat(modelPath); err != nil {
			if os.IsNotExist(err) {
				t.Skip("Mozilla EN-DE model is not downloaded; see the README smoke-test instructions")
			}
			t.Fatalf("Mozilla EN-DE model %q: %v", modelPath, err)
		}
		configPath = filepath.Join(projectRoot, "pkg", "testdata", "bridge-smoke-mozilla-en-de.yml")
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(projectRoot, configPath)
	}
	if _, err := os.Stat(configPath); err != nil {
		if os.Getenv("BERGAMOT_TEST_MODEL_CONFIG") == "" && os.IsNotExist(err) {
			t.Skipf("Bergamot regression model is not downloaded; run the models target in third_party/bergamot-translator/bergamot-translator-tests or set BERGAMOT_TEST_MODEL_CONFIG")
		}
		t.Fatalf("Bergamot model config %q: %v", configPath, err)
	}

	bridge, err := Init(configPath)
	if err != nil {
		t.Fatalf("initialize Bergamot model: %v", err)
	}
	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Errorf("clean up Bergamot model: %v", err)
		}
	})

	input := os.Getenv("BERGAMOT_TEST_INPUT")
	if input == "" {
		fixturePath := filepath.Join(projectRoot, "third_party", "bergamot-translator", "bergamot-translator-tests", "data", "simple", "bergamot", "input.txt")
		fixture, err := os.ReadFile(fixturePath)
		if err != nil {
			t.Fatalf("read Bergamot translation fixture %q: %v", fixturePath, err)
		}
		input = strings.TrimSpace(strings.SplitN(string(fixture), "\n", 2)[0])
		if input == "" {
			t.Fatalf("Bergamot translation fixture %q is empty", fixturePath)
		}
	}
	translation, err := bridge.Translate(input)
	if err != nil {
		t.Fatalf("translate %q: %v", input, err)
	}
	if strings.TrimSpace(translation) == "" {
		t.Fatal("Bergamot returned an empty translation")
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("clean up Bergamot model: %v", err)
	}
}
