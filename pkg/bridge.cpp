#if defined(__ARM_NEON) && !defined(ARM)
#include <fenv.h>
#define ARM
#endif

#include "bridge.h"

#include <cstdlib>
#include <cstring>
#include <exception>
#include <fstream>
#include <memory>
#include <string>
#include <utility>
#include <vector>

#include <common/logging.h>
#include <translator/parser.h>
#include <translator/response.h>
#include <translator/response_options.h>
#include <translator/service.h>
#include <translator/translation_model.h>

struct BergamotHandle {
  marian::bergamot::BlockingService service;
  std::shared_ptr<marian::bergamot::TranslationModel> model;

  BergamotHandle(const marian::bergamot::BlockingService::Config &serviceConfig,
                 const std::shared_ptr<marian::bergamot::TranslationModel> &translationModel)
      : service(serviceConfig), model(translationModel) {}
};

namespace {

void setError(char **errorOut, const char *message) noexcept {
  if (!errorOut) return;
  *errorOut = nullptr;
  const char *text = message ? message : "unknown Bergamot bridge error";
  const size_t bytes = std::strlen(text) + 1;
  char *copy = static_cast<char *>(std::malloc(bytes));
  if (copy) std::memcpy(copy, text, bytes);
  *errorOut = copy;
}

void clearError(char **errorOut) noexcept {
  if (errorOut) *errorOut = nullptr;
}

}  // namespace

extern "C" BergamotHandle *bergamot_init(const char *model_config_path, char **error_out) noexcept {
  clearError(error_out);
  try {
    if (!model_config_path || model_config_path[0] == '\0') {
      setError(error_out, "model config path must not be empty");
      return nullptr;
    }
    std::ifstream configFile(model_config_path);
    if (!configFile) {
      const std::string message = std::string("unable to open model config: ") + model_config_path;
      setError(error_out, message.c_str());
      return nullptr;
    }

    // Model creation uses Marian's abort-to-exception mode so C++ failures can
    // be returned to callers instead of terminating the Go process.
    marian::setThrowExceptionOnAbort(true);
    auto config = marian::bergamot::parseOptionsFromFilePath(model_config_path);
    marian::bergamot::BlockingService::Config serviceConfig;
    auto model = std::make_shared<marian::bergamot::TranslationModel>(config);
    return new BergamotHandle(serviceConfig, model);
  } catch (const std::exception &error) {
    setError(error_out, error.what());
    return nullptr;
  } catch (...) {
    setError(error_out, "unknown C++ exception while initializing Bergamot");
    return nullptr;
  }
}

extern "C" char *bergamot_translate(BergamotHandle *handle, const char *input, size_t input_length, char **error_out) noexcept {
  clearError(error_out);
  try {
    if (!handle) {
      setError(error_out, "Bergamot handle is null");
      return nullptr;
    }
    if (!input && input_length != 0) {
      setError(error_out, "translation input is null");
      return nullptr;
    }

    std::vector<std::string> inputs{std::string(input ? input : "", input_length)};
    std::vector<marian::bergamot::ResponseOptions> options(1);
    auto responses = handle->service.translateMultiple(handle->model, std::move(inputs), options);
    if (responses.empty()) {
      setError(error_out, "Bergamot returned no translation response");
      return nullptr;
    }

    const std::string &translation = responses.front().getTranslatedText();
    char *result = static_cast<char *>(std::malloc(translation.size() + 1));
    if (!result) {
      setError(error_out, "unable to allocate translation result");
      return nullptr;
    }
    std::memcpy(result, translation.c_str(), translation.size() + 1);
    return result;
  } catch (const std::exception &error) {
    setError(error_out, error.what());
    return nullptr;
  } catch (...) {
    setError(error_out, "unknown C++ exception while translating");
    return nullptr;
  }
}

extern "C" void bergamot_string_free(char *value) noexcept { std::free(value); }

extern "C" void bergamot_cleanup(BergamotHandle *handle) noexcept {
  try {
    delete handle;
  } catch (...) {
    // Cleanup has no error result by design; never unwind through the C ABI.
  }
}
