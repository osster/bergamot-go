#ifndef BERGAMOT_GO_BRIDGE_H_
#define BERGAMOT_GO_BRIDGE_H_

#ifdef __cplusplus
#define BERGAMOT_BRIDGE_NOEXCEPT noexcept
extern "C" {
#else
#define BERGAMOT_BRIDGE_NOEXCEPT
#endif

#include <stddef.h>

/* Opaque owner of a Bergamot BlockingService and its TranslationModel.
 * A successful bergamot_init transfers ownership of the returned handle to the
 * caller; destroy it exactly once with bergamot_cleanup. The handle owns both
 * C++ objects; callers must not copy or free it themselves. Keep it alive and
 * do not use it concurrently with cleanup or another call.
 */
typedef struct BergamotHandle BergamotHandle;

/* Initialize a CPU translation model from Bergamot's YAML/JSON model config.
 * model_config_path must be a valid NUL-terminated string and is borrowed only
 * for this call; the caller retains ownership. On success this returns a new
 * opaque handle owned by the caller (release it exactly once with
 * bergamot_cleanup) and sets *error_out to NULL. On failure it returns NULL
 * and, if error_out is non-NULL, stores an allocated UTF-8 error message
 * owned by the caller; release that message with bergamot_string_free. If
 * allocating the message fails, *error_out is NULL. error_out itself is an
 * optional, caller-owned writable pointer slot. No C++ exception escapes.
 */
BergamotHandle *bergamot_init(const char *model_config_path, char **error_out) BERGAMOT_BRIDGE_NOEXCEPT;

/* Translate one UTF-8 input string. input points to input_length readable
 * bytes and is borrowed only for this call; the caller retains ownership.
 * On success this returns an allocated NUL-terminated UTF-8 buffer owned by
 * the caller, which must release it with bergamot_string_free, and sets
 * *error_out to NULL. On failure it returns NULL and stores an allocated
 * message in *error_out when that slot is non-NULL; the caller owns and must
 * release the message with bergamot_string_free. If message allocation fails,
 * *error_out is NULL. The handle must remain alive and unused by cleanup for
 * the full call. No C++ exception escapes.
 */
char *bergamot_translate(BergamotHandle *handle, const char *input, size_t input_length, char **error_out) BERGAMOT_BRIDGE_NOEXCEPT;

/* Release a translation buffer or error message returned by this API.
 * NULL is accepted; callers must not use the buffer after releasing it.
 */
void bergamot_string_free(char *value) BERGAMOT_BRIDGE_NOEXCEPT;

/* Destroy the service and model owned by handle. NULL is accepted. After this
 * call the handle is invalid and must not be used or cleaned up again. This
 * operation has no error result; the implementation contains native failures
 * rather than allowing a C++ exception to unwind through the C ABI.
 */
void bergamot_cleanup(BergamotHandle *handle) BERGAMOT_BRIDGE_NOEXCEPT;

#ifdef __cplusplus
} /* extern "C" */
#endif

#undef BERGAMOT_BRIDGE_NOEXCEPT

#endif /* BERGAMOT_GO_BRIDGE_H_ */
