#ifndef BERGAMOT_GO_BRIDGE_H_
#define BERGAMOT_GO_BRIDGE_H_

#ifdef __cplusplus
extern "C" {
#endif

/* Opaque owner of a Bergamot BlockingService and its TranslationModel.
 * Create with bergamot_init and destroy exactly once with bergamot_cleanup.
 * The handle owns both C++ objects; callers must not copy or free it themselves.
 */
typedef struct BergamotHandle BergamotHandle;

/* Initialize a CPU translation model from Bergamot's YAML/JSON model config.
 * On success returns a handle and sets *error_out to NULL. On failure returns
 * NULL; when error_out is non-NULL, *error_out is an allocated UTF-8 message.
 */
BergamotHandle *bergamot_init(const char *model_config_path, char **error_out);

/* Translate one UTF-8 input string. The returned output is an allocated
 * NUL-terminated UTF-8 buffer; the caller owns it and releases it with
 * bergamot_string_free. On failure returns NULL and sets *error_out to an
 * allocated message (released by the same function). Input is borrowed only
 * for the duration of this call. The handle must remain alive for the call.
 */
char *bergamot_translate(BergamotHandle *handle, const char *input, char **error_out);

/* Release a string or error message returned by this API. NULL is accepted. */
void bergamot_string_free(char *value);

/* Destroy the service and model owned by handle. NULL is accepted. */
void bergamot_cleanup(BergamotHandle *handle);

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* BERGAMOT_GO_BRIDGE_H_ */
