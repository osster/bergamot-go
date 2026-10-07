package bergamot

import (
	"errors"
	"sync"
)

type modelPoolKey struct {
	modelConfig string
	beamSize    int
}

type sharedModel struct {
	mu     sync.Mutex
	key    modelPoolKey
	bridge translationBridge
	refs   int
}

type sharedBridge struct {
	mu    sync.Mutex
	model *sharedModel
}

var translatorModelPool = struct {
	sync.Mutex
	models map[modelPoolKey]*sharedModel
}{models: make(map[modelPoolKey]*sharedModel)}

var translatorModelInitMu sync.Mutex

func acquireSharedBridge(key modelPoolKey, initialize bridgeInitializer) (translationBridge, error) {
	translatorModelPool.Lock()
	if model := translatorModelPool.models[key]; model != nil {
		model.refs++
		translatorModelPool.Unlock()
		return newSharedBridge(model), nil
	}
	translatorModelPool.Unlock()

	// Serialize only first-time model creation; translations and releases for
	// already-loaded models remain independent of this potentially slow operation.
	translatorModelInitMu.Lock()
	defer translatorModelInitMu.Unlock()

	translatorModelPool.Lock()
	if model := translatorModelPool.models[key]; model != nil {
		model.refs++
		translatorModelPool.Unlock()
		return newSharedBridge(model), nil
	}
	translatorModelPool.Unlock()

	bridge, err := initialize(key.modelConfig, key.beamSize)
	if err != nil {
		return nil, err
	}
	if bridge == nil {
		return nil, errors.New("initializer returned a nil bridge")
	}

	model := &sharedModel{key: key, bridge: bridge, refs: 1}
	translatorModelPool.Lock()
	translatorModelPool.models[key] = model
	translatorModelPool.Unlock()
	return newSharedBridge(model), nil
}

func newSharedBridge(model *sharedModel) *sharedBridge {
	bridge := &sharedBridge{model: model}
	return bridge
}

func (b *sharedBridge) Translate(input string) (string, error) {
	return b.translate("", input)
}

func (b *sharedBridge) TranslateWithContext(context, input string) (string, error) {
	return b.translate(context, input)
}

func (b *sharedBridge) translate(context, input string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.model == nil {
		return "", errors.New("shared Bergamot model is closed")
	}
	model := b.model
	model.mu.Lock()
	defer model.mu.Unlock()
	if contextual, ok := model.bridge.(contextualTranslationBridge); ok {
		return contextual.TranslateWithContext(context, input)
	}
	return model.bridge.Translate(input)
}

func (b *sharedBridge) TranslateMultiple(inputs []string) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.model == nil {
		return nil, errors.New("shared Bergamot model is closed")
	}
	model := b.model
	model.mu.Lock()
	defer model.mu.Unlock()
	return translateMultiple(model.bridge, inputs)
}

func (b *sharedBridge) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	model := b.model
	b.model = nil
	b.mu.Unlock()
	if model == nil {
		return nil
	}

	translatorModelPool.Lock()
	model.refs--
	if model.refs > 0 {
		translatorModelPool.Unlock()
		return nil
	}
	if translatorModelPool.models[model.key] == model {
		delete(translatorModelPool.models, model.key)
	}
	translatorModelPool.Unlock()

	model.mu.Lock()
	err := model.bridge.Close()
	model.bridge = nil
	model.mu.Unlock()
	return err
}
