package bergamot

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
)

// TranslationResult contains the outcome of one asynchronous translation.
type TranslationResult struct {
	Text string
	Err  error
}

// TranslateAsync starts a translation in a goroutine and returns a channel
// that receives exactly one result before it is closed. Cancellation prevents
// work that has not started, but cannot interrupt a native translation already
// in progress.
func (t *Translator) TranslateAsync(ctx context.Context, text, languagePair string) <-chan TranslationResult {
	return t.translateAsync(ctx, text, languagePair, nil)
}

// TranslateAsyncWithCallback starts an asynchronous translation and invokes
// callback once with the same result delivered on the returned channel. The
// callback runs in the worker goroutine after the buffered result is available.
func (t *Translator) TranslateAsyncWithCallback(ctx context.Context, text, languagePair string, callback func(TranslationResult)) <-chan TranslationResult {
	return t.translateAsync(ctx, text, languagePair, callback)
}

func (t *Translator) translateAsync(ctx context.Context, text, languagePair string, callback func(TranslationResult)) <-chan TranslationResult {
	results := make(chan TranslationResult, 1)
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		defer close(results)
		var result TranslationResult
		select {
		case <-ctx.Done():
			result.Err = newError(ErrOperationCanceled, "translate async", languagePair, ctx.Err())
		default:
			result.Text, result.Err = t.Translate(text, languagePair)
		}
		results <- result
		if callback != nil {
			callback(result)
		}
	}()
	return results
}

// StreamProgress reports completed chunks and bytes consumed from the input.
// ChunkIndex is one-based; ChunksCompleted does not include a failed chunk.
type StreamProgress struct {
	ChunkIndex      int
	ChunksCompleted int
	BytesProcessed  int64
}

// StreamResult contains one translated input chunk and its cumulative
// progress, or a terminal error. Results arrive in input order.
type StreamResult struct {
	Text     string
	Progress StreamProgress
	Err      error
}

// TranslateStream reads text from input and translates it in chunks of at most
// chunkRunes Unicode code points. The channel is closed when input is exhausted,
// an error occurs, or ctx is canceled. Cancellation is checked between chunks;
// it cannot interrupt a native translation already in progress. Progress is
// cumulative and BytesProcessed counts UTF-8 bytes read from input.
func (t *Translator) TranslateStream(ctx context.Context, input io.Reader, languagePair string, chunkRunes int) <-chan StreamResult {
	results := make(chan StreamResult, 1)
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		defer close(results)
		if chunkRunes <= 0 {
			sendStreamResult(ctx, results, StreamResult{Err: newError(ErrInvalidChunkSize, "stream translate", languagePair, nil)})
			return
		}
		if input == nil {
			sendStreamResult(ctx, results, StreamResult{Err: newError(ErrStreamRead, "read translation stream", languagePair, errors.New("reader must not be nil"))})
			return
		}

		reader := bufio.NewReader(input)
		var chunk strings.Builder
		chunkRuneCount := 0
		bytesRead := int64(0)
		chunksCompleted := 0
		for {
			if err := ctx.Err(); err != nil {
				sendStreamResult(ctx, results, StreamResult{Progress: StreamProgress{ChunkIndex: chunksCompleted + 1, ChunksCompleted: chunksCompleted, BytesProcessed: bytesRead}, Err: newError(ErrOperationCanceled, "stream translate", languagePair, err)})
				return
			}

			r, size, readErr := reader.ReadRune()
			if readErr == nil {
				chunk.WriteRune(r)
				chunkRuneCount++
				bytesRead += int64(size)
			}

			flush := chunkRuneCount == chunkRunes || readErr != nil
			if flush && chunkRuneCount > 0 {
				text := chunk.String()
				chunk.Reset()
				chunkRuneCount = 0
				translation := text
				var err error
				if strings.TrimSpace(text) != "" {
					translation, err = t.Translate(text, languagePair)
				}
				progress := StreamProgress{ChunkIndex: chunksCompleted + 1, ChunksCompleted: chunksCompleted, BytesProcessed: bytesRead}
				if err != nil {
					sendStreamResult(ctx, results, StreamResult{Progress: progress, Err: err})
					return
				}
				chunksCompleted++
				progress.ChunksCompleted = chunksCompleted
				if !sendStreamResult(ctx, results, StreamResult{Text: translation, Progress: progress}) {
					return
				}
			}

			if readErr != nil {
				if readErr == io.EOF {
					if chunksCompleted == 0 {
						sendStreamResult(ctx, results, StreamResult{Err: newError(ErrEmptyInput, "stream translate", languagePair, nil)})
					}
					return
				}
				sendStreamResult(ctx, results, StreamResult{Progress: StreamProgress{ChunkIndex: chunksCompleted + 1, ChunksCompleted: chunksCompleted, BytesProcessed: bytesRead}, Err: newError(ErrStreamRead, "read translation stream", languagePair, readErr)})
				return
			}
		}
	}()
	return results
}

func sendStreamResult(ctx context.Context, results chan<- StreamResult, result StreamResult) bool {
	select {
	case results <- result:
		return true
	default:
	}
	select {
	case results <- result:
		return true
	case <-ctx.Done():
		return false
	}
}
