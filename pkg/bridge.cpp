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
#include <mutex>
#include <string>
#include <utility>
#include <vector>

#include <common/logging.h>
#include <translator/parser.h>
#include <translator/response.h>
#include <translator/response_options.h>
#include <translator/service.h>
#include <translator/translation_model.h>

// A BlockingService owns Marian's process-global "general" and "valid"
// loggers, so a second service cannot exist while one is alive. All handles
// therefore share one service, which serves any number of models. The service
// is not thread-safe: every use, and its creation and destruction, happen under
// serviceMutex.
struct BergamotHandle {
  std::shared_ptr<marian::bergamot::BlockingService> service;
  std::shared_ptr<marian::bergamot::TranslationModel> model;

  BergamotHandle(const std::shared_ptr<marian::bergamot::BlockingService> &sharedService,
                 const std::shared_ptr<marian::bergamot::TranslationModel> &translationModel)
      : service(sharedService), model(translationModel) {}
};

namespace {

std::mutex serviceMutex;
// The last handle to close destroys the service and drops its loggers, so a
// later bergamot_init starts from a clean logger registry.
std::weak_ptr<marian::bergamot::BlockingService> sharedService;

// acquireService returns the shared service, creating it when no handle holds
// one. The caller must hold serviceMutex.
std::shared_ptr<marian::bergamot::BlockingService> acquireService() {
  auto service = sharedService.lock();
  if (!service) {
    marian::bergamot::BlockingService::Config serviceConfig;
    service = std::make_shared<marian::bergamot::BlockingService>(serviceConfig);
    sharedService = service;
  }
  return service;
}

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

extern "C" BergamotHandle *bergamot_init(const char *model_config_path, int beam_size, char **error_out) noexcept {
  clearError(error_out);
  try {
    if (!model_config_path || model_config_path[0] == '\0') {
      setError(error_out, "model config path must not be empty");
      return nullptr;
    }
    if (beam_size < 0) {
      setError(error_out, "beam size must not be negative");
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
    if (beam_size > 0) config->set("beam-size", static_cast<size_t>(beam_size));
    std::lock_guard<std::mutex> lock(serviceMutex);
    auto service = acquireService();
    auto model = std::make_shared<marian::bergamot::TranslationModel>(config);
    return new BergamotHandle(service, model);
  } catch (const std::exception &error) {
    setError(error_out, error.what());
    return nullptr;
  } catch (...) {
    setError(error_out, "unknown C++ exception while initializing Bergamot");
    return nullptr;
  }
}

extern "C" char *bergamot_translate(BergamotHandle *handle, const char *context, size_t context_length,
                                    const char *input, size_t input_length, char **error_out) noexcept {
  clearError(error_out);
  try {
    if (!handle) {
      setError(error_out, "Bergamot handle is null");
      return nullptr;
    }
    if ((!context && context_length != 0) || (!input && input_length != 0)) {
      setError(error_out, "translation input is null");
      return nullptr;
    }

    std::string source;
    if (context_length != 0) {
      source.assign(context, context_length);
      source.push_back('\n');
    }
    const size_t currentInputStart = source.size();
    source.append(input ? input : "", input_length);
    std::vector<std::string> inputs{std::move(source)};
    std::vector<marian::bergamot::ResponseOptions> options(1);
    std::vector<marian::bergamot::Response> responses;
    {
      std::lock_guard<std::mutex> lock(serviceMutex);
      responses = handle->service->translateMultiple(handle->model, std::move(inputs), options);
    }
    if (responses.empty()) {
      setError(error_out, "Bergamot returned no translation response");
      return nullptr;
    }

    const auto &response = responses.front();
    const std::string &translation = response.getTranslatedText();
    size_t translatedStart = 0;
    if (context_length != 0) {
      size_t currentSentence = response.size();
      for (size_t i = 0; i < response.size(); ++i) {
        if (response.getSourceSentenceAsByteRange(i).begin >= currentInputStart) {
          currentSentence = i;
          break;
        }
      }
      if (currentSentence == response.size()) {
        setError(error_out, "Bergamot did not return a sentence for the current input");
        return nullptr;
      }
      translatedStart = response.getTargetSentenceAsByteRange(currentSentence).begin;
      if (translatedStart > translation.size()) {
        setError(error_out, "Bergamot returned an invalid translation sentence range");
        return nullptr;
      }
    }
    const size_t currentTranslationLength = translation.size() - translatedStart;
    char *result = static_cast<char *>(std::malloc(currentTranslationLength + 1));
    if (!result) {
      setError(error_out, "unable to allocate translation result");
      return nullptr;
    }
    std::memcpy(result, translation.data() + translatedStart, currentTranslationLength);
    result[currentTranslationLength] = '\0';
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
    // Destroying the last handle destroys the shared service, which must not
    // overlap another handle creating a new one.
    std::lock_guard<std::mutex> lock(serviceMutex);
    delete handle;
  } catch (...) {
    // Cleanup has no error result by design; never unwind through the C ABI.
  }
}
