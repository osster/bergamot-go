package bergamot

/*
#cgo CPPFLAGS: -I${SRCDIR} -I${SRCDIR}/../third_party/bergamot-translator -I${SRCDIR}/../third_party/bergamot-translator/src -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/SQLiteCpp/include -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/sentencepiece -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/sentencepiece/third_party/protobuf-lite -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/fbgemm/include -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/intgemm -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/ruy -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/ruy/third_party/cpuinfo/include -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/ruy/third_party/cpuinfo/deps/clog/include -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/ssplit-cpp/src/ssplit -I${SRCDIR}/../third_party/bergamot-translator/3rd_party/ssplit-cpp/src/3rd-party/CLI11 -I${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party/intgemm -I${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party
#cgo CXXFLAGS: -std=c++17
#cgo darwin,arm64 CXXFLAGS: -DARM -DBLAS_FOUND=1 -DCOMPILE_CPU=1 -DCPUINFO_SUPPORTED_PLATFORM=1 -DFMA -DSSE -DUSE_PTHREADS
#cgo LDFLAGS: -L${SRCDIR}/../build/bergamot/src/translator -lbergamot-translator -L${SRCDIR}/../build/bergamot -lmarian -L${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party/sentencepiece/src -lsentencepiece_train -lsentencepiece -L${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party/ruy/ruy -lruy_context_get_ctx -lruy_context -lruy_frontend -lruy_kernel_arm -lruy_kernel_avx -lruy_kernel_avx2_fma -lruy_kernel_avx512 -lruy_apply_multiplier -lruy_pack_arm -lruy_pack_avx -lruy_pack_avx2_fma -lruy_pack_avx512 -lruy_prepare_packed_matrices -lruy_trmul -lruy_ctx -lruy_allocator -lruy_prepacked_cache -lruy_system_aligned_alloc -lruy_have_built_path_for_avx -lruy_have_built_path_for_avx2_fma -lruy_have_built_path_for_avx512 -lruy_thread_pool -lruy_blocking_counter -lruy_wait -lruy_denormal -lruy_block_map -lruy_tune -lruy_cpuinfo -L${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party/ruy/third_party/cpuinfo -lcpuinfo -L${SRCDIR}/../build/bergamot/3rd_party/marian-dev/src/3rd_party/ruy/third_party/cpuinfo/deps/clog -lclog -L${SRCDIR}/../build/bergamot -lssplit -lpcre2-8 -lz -pthread
#cgo linux LDFLAGS: -lstdc++
#cgo darwin LDFLAGS: -framework Accelerate -liconv -lc++
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

// Bridge owns a native Bergamot service/model handle. It must be closed once
// no further Translate calls will be made. Calls on one Bridge are not safe to
// run concurrently; synchronization is deliberately left to higher-level APIs.
type Bridge struct {
	handle *C.BergamotHandle
}

// Init loads a Bergamot translation model using its model configuration file.
func Init(modelConfigPath string) (*Bridge, error) {
	path := C.CString(modelConfigPath)
	defer C.free(unsafe.Pointer(path))

	var cErr *C.char
	handle := C.bergamot_init(path, &cErr)
	if handle == nil {
		return nil, takeError(cErr, "Bergamot initialization failed")
	}
	if cErr != nil {
		C.bergamot_string_free(cErr)
	}
	return &Bridge{handle: handle}, nil
}

// Translate translates one UTF-8 string through the initialized model. The
// input bytes are borrowed by C++ only for the duration of the native call.
func (b *Bridge) Translate(input string) (string, error) {
	if b == nil || b.handle == nil {
		return "", errors.New("Bergamot bridge is closed")
	}
	var text *C.char
	if len(input) != 0 {
		text = (*C.char)(unsafe.Pointer(unsafe.StringData(input)))
	}
	translation, err := translateNative(b.handle, text, C.size_t(len(input)))
	runtime.KeepAlive(input)
	return translation, err
}

// translateNative owns and releases every buffer returned by the C API. Keeping
// this conversion in one place ensures native failures never produce a result.
func translateNative(handle *C.BergamotHandle, input *C.char, inputLength C.size_t) (string, error) {
	var cErr *C.char
	result := C.bergamot_translate(handle, input, inputLength, &cErr)
	if result == nil {
		return "", takeError(cErr, "Bergamot translation failed")
	}
	defer C.bergamot_string_free(result)
	if cErr != nil {
		C.bergamot_string_free(cErr)
	}
	return C.GoString(result), nil
}

// translateWithNullHandle exercises the native translation-failure path without
// requiring a downloaded model. It shares the exact wrapper used by Translate.
func translateWithNullHandle() (string, error) {
	input := C.CString("failure-path-test")
	defer C.free(unsafe.Pointer(input))
	return translateNative(nil, input, C.size_t(len("failure-path-test")))
}

// Close releases the native service and model. It is safe to call on a nil or
// already-closed Bridge.
func (b *Bridge) Close() error {
	if b != nil && b.handle != nil {
		C.bergamot_cleanup(b.handle)
		b.handle = nil
	}
	return nil
}

func takeError(message *C.char, fallback string) error {
	if message == nil {
		return errors.New(fallback)
	}
	defer C.bergamot_string_free(message)
	return fmt.Errorf("%s", C.GoString(message))
}
