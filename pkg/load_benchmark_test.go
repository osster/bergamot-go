package bergamot

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

const benchmarkLanguagePairEnv = "BERGAMOT_BENCH_LANGUAGE_PAIR"

func benchmarkTranslator(b *testing.B) (*Translator, string) {
	b.Helper()
	configPath := os.Getenv("BERGAMOT_BENCH_CONFIG")
	if configPath == "" {
		b.Skip("set BERGAMOT_BENCH_CONFIG to a translator language-pair YAML config")
	}
	pair := os.Getenv(benchmarkLanguagePairEnv)
	if pair == "" {
		pair = "en-de"
	}
	translator, err := NewTranslator(configPath)
	if err != nil {
		b.Fatalf("create benchmark translator: %v", err)
	}
	b.Cleanup(func() {
		if err := translator.Close(); err != nil {
			b.Errorf("close benchmark translator: %v", err)
		}
	})
	if _, err := translator.Translate("Warm up the translation model.", pair); err != nil {
		b.Fatalf("warm benchmark translator for %s: %v", pair, err)
	}
	return translator, pair
}

func BenchmarkLoadTranslate(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	inputs := map[string]string{
		"short":  "Good morning.",
		"medium": "Good morning. This is a realistic sentence for measuring translation throughput.",
		"long":   strings.Repeat("This is a representative sentence for a longer document. ", 40),
	}
	for name, input := range inputs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := translator.Translate(input, pair); err != nil {
					b.Fatalf("translate: %v", err)
				}
			}
			reportOperationsPerSecond(b)
		})
	}
}

func BenchmarkLoadTranslateAsync(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	input := "Good morning. This request uses the asynchronous translation API."
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := <-translator.TranslateAsync(context.Background(), input, pair)
		if result.Err != nil {
			b.Fatalf("translate asynchronously: %v", result.Err)
		}
	}
	reportOperationsPerSecond(b)
}

func BenchmarkLoadTranslateCallback(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	input := "Good morning. This request uses the asynchronous translation API."
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callbackDone := make(chan struct{})
		result := <-translator.TranslateAsyncWithCallback(context.Background(), input, pair, func(TranslationResult) {
			close(callbackDone)
		})
		if result.Err != nil {
			b.Fatalf("translate with callback: %v", result.Err)
		}
		<-callbackDone
	}
	reportOperationsPerSecond(b)
}

func BenchmarkLoadTranslateStream(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	input := strings.Repeat("Streaming translates large documents incrementally. ", 40)
	for _, chunkRunes := range []int{128, 512, 2048} {
		b.Run(fmt.Sprintf("chunk_%d", chunkRunes), func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for result := range translator.TranslateStream(context.Background(), strings.NewReader(input), pair, chunkRunes) {
					if result.Err != nil {
						b.Fatalf("translate stream: %v", result.Err)
					}
				}
			}
			reportOperationsPerSecond(b)
		})
	}
}

func BenchmarkLoadTranslateContext(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	if translator.pairSettings[normalizeLanguagePair(pair)].contextWindow == 0 {
		b.Skip("enable context_window for the selected language pair to benchmark rolling context")
	}
	input := "The next sentence uses the preceding source context."
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := translator.Translate(input, pair); err != nil {
			b.Fatalf("translate with rolling context: %v", err)
		}
	}
	reportOperationsPerSecond(b)
}

func BenchmarkLoadTranslateParallel(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	inputs := map[string]string{
		"short": "Good morning.",
		"long":  strings.Repeat("This is a representative sentence for a longer document. ", 40),
	}
	for name, input := range inputs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(input)))
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					if _, err := translator.Translate(input, pair); err != nil {
						b.Errorf("translate concurrently: %v", err)
						return
					}
				}
			})
			reportOperationsPerSecond(b)
		})
	}
}

func BenchmarkLoadTranslateSeparateTranslators(b *testing.B) {
	translator, pair := benchmarkTranslator(b)
	translators := make([]*Translator, runtime.GOMAXPROCS(0))
	translators[0] = translator
	for i := 1; i < len(translators); i++ {
		other, err := NewTranslator(os.Getenv("BERGAMOT_BENCH_CONFIG"))
		if err != nil {
			b.Fatalf("create translator %d: %v", i, err)
		}
		if _, err := other.Translate("Warm up the translation model.", pair); err != nil {
			b.Fatalf("warm translator %d: %v", i, err)
		}
		translators[i] = other
		translatorIndex := i
		b.Cleanup(func() {
			if err := other.Close(); err != nil {
				b.Errorf("close translator %d: %v", translatorIndex, err)
			}
		})
	}
	input := "Good morning."
	var next uint64
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			index := atomic.AddUint64(&next, 1) - 1
			if _, err := translators[index%uint64(len(translators))].Translate(input, pair); err != nil {
				b.Errorf("translate with separate translator: %v", err)
				return
			}
		}
	})
	reportOperationsPerSecond(b)
}

func reportOperationsPerSecond(b *testing.B) {
	b.Helper()
	if elapsed := b.Elapsed().Seconds(); elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed, "ops/s")
	}
}
