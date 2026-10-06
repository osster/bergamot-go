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
		modelDir := filepath.Join(projectRoot, "third_party", "bergamot-translator", "models", "mozilla-en-de")
		artifacts := []string{
			"model.ende.intgemm.alphas.bin",
			"vocab.ende.spm",
			"lex.50.50.ende.s2t.bin",
		}
		for _, artifact := range artifacts {
			modelPath := filepath.Join(modelDir, artifact)
			if _, err := os.Stat(modelPath); err != nil {
				if os.IsNotExist(err) {
					t.Skipf("Mozilla EN-DE model artifact %q is not downloaded; follow doc/BUILD_AND_TEST.md for the end-to-end bridge smoke-test instructions", modelPath)
				}
				t.Fatalf("Mozilla EN-DE model artifact %q: %v", modelPath, err)
			}
		}
		configPath = filepath.Join(projectRoot, "pkg", "testdata", "bridge-smoke-mozilla-en-de.yml")
		configPath = writeRelocatedModelConfig(t, configPath, modelDir, artifacts)
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
		for _, line := range strings.Split(string(fixture), "\n") {
			if input = strings.TrimSpace(line); input != "" {
				break
			}
		}
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
	contextTranslation, err := bridge.TranslateWithContext(input, input)
	if err != nil {
		t.Fatalf("translate with source context: %v", err)
	}
	if contextTranslation != translation {
		t.Errorf("contextual translation = %q, want current-input translation %q", contextTranslation, translation)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("clean up Bergamot model: %v", err)
	}
	if err := bridge.Close(); err != nil {
		t.Fatalf("close Bergamot model a second time: %v", err)
	}
	if _, err := bridge.Translate(input); err == nil {
		t.Fatal("Translate after cleanup succeeded; want a closed-bridge error")
	}

	reloadedBridge, err := initWithBeamSize(configPath, 4)
	if err != nil {
		t.Fatalf("reload Bergamot model after cleanup: %v", err)
	}
	t.Cleanup(func() {
		if err := reloadedBridge.Close(); err != nil {
			t.Errorf("clean up reloaded Bergamot model: %v", err)
		}
	})
	reloadedTranslation, err := reloadedBridge.Translate(input)
	if err != nil {
		t.Fatalf("translate after model reload: %v", err)
	}
	if strings.TrimSpace(reloadedTranslation) == "" {
		t.Fatal("reloaded Bergamot model returned an empty translation")
	}
}

func writeRelocatedModelConfig(t *testing.T, sourceConfigPath, modelDir string, artifacts []string) string {
	t.Helper()

	contents, err := os.ReadFile(sourceConfigPath)
	if err != nil {
		t.Fatalf("read Bergamot model config %q: %v", sourceConfigPath, err)
	}
	tempDir := t.TempDir()
	replacements := make([]string, 0, len(artifacts)*2)
	for _, artifact := range artifacts {
		artifactPath := filepath.Join(modelDir, artifact)
		sourceRelativePath, err := filepath.Rel(filepath.Dir(sourceConfigPath), artifactPath)
		if err != nil {
			t.Fatalf("make source artifact path relative: %v", err)
		}
		tempRelativePath, err := filepath.Rel(tempDir, artifactPath)
		if err != nil {
			t.Fatalf("make temporary artifact path relative: %v", err)
		}
		sourceRelativePath = filepath.ToSlash(sourceRelativePath)
		if !strings.Contains(string(contents), sourceRelativePath) {
			t.Fatalf("model config %q does not reference artifact %q", sourceConfigPath, sourceRelativePath)
		}
		replacements = append(replacements, sourceRelativePath, filepath.ToSlash(tempRelativePath))
	}
	contents = []byte(strings.NewReplacer(replacements...).Replace(string(contents)))
	configPath := filepath.Join(tempDir, filepath.Base(sourceConfigPath))
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatalf("write relocated Bergamot model config %q: %v", configPath, err)
	}
	return configPath
}

// TestBridgeSmokeMultipleModels keeps two different models loaded at once and
// translates with both, alternating, then closes them in creation order. Set
// BERGAMOT_TEST_MODEL_CONFIGS to two or more comma-separated model configs.
func TestBridgeSmokeMultipleModels(t *testing.T) {
	configs := strings.Split(os.Getenv("BERGAMOT_TEST_MODEL_CONFIGS"), ",")
	if len(configs) < 2 || configs[0] == "" {
		t.Skip("set BERGAMOT_TEST_MODEL_CONFIGS to two or more comma-separated model configs")
	}
	input := os.Getenv("BERGAMOT_TEST_INPUT")
	if input == "" {
		input = "Hello, world."
	}

	bridges := make([]*Bridge, 0, len(configs))
	for _, configPath := range configs {
		bridge, err := Init(strings.TrimSpace(configPath))
		if err != nil {
			t.Fatalf("initialize Bergamot model %q while %d other models are loaded: %v", configPath, len(bridges), err)
		}
		bridges = append(bridges, bridge)
	}
	for round := 0; round < 2; round++ {
		for i, bridge := range bridges {
			translation, err := bridge.Translate(input)
			if err != nil {
				t.Fatalf("translate with model %q: %v", configs[i], err)
			}
			if strings.TrimSpace(translation) == "" {
				t.Fatalf("model %q returned an empty translation", configs[i])
			}
		}
	}

	// Closing the first model must not affect the models still loaded.
	if err := bridges[0].Close(); err != nil {
		t.Fatalf("clean up Bergamot model %q: %v", configs[0], err)
	}
	if _, err := bridges[1].Translate(input); err != nil {
		t.Fatalf("translate with model %q after closing another model: %v", configs[1], err)
	}
	for i, bridge := range bridges[1:] {
		if err := bridge.Close(); err != nil {
			t.Fatalf("clean up Bergamot model %q: %v", configs[i+1], err)
		}
	}

	// Once every model is closed, a new model must initialize again.
	reloaded, err := Init(strings.TrimSpace(configs[0]))
	if err != nil {
		t.Fatalf("reinitialize Bergamot model %q after closing all models: %v", configs[0], err)
	}
	defer reloaded.Close()
	if _, err := reloaded.Translate(input); err != nil {
		t.Fatalf("translate after reinitialization: %v", err)
	}
}
