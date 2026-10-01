// Package bergamot provides Go APIs for translating text with Mozilla's
// Bergamot neural machine translation engine.
//
// The package requires cgo and a native Bergamot build. Models and their
// artifacts are supplied by the application; model downloading is not
// performed by this package. NewTranslator reads a YAML or JSON language-pair
// configuration and loads each native model lazily on its first translation.
//
// Call Close when finished to release native resources. Translators are safe
// for concurrent use, although requests using a shared native model are
// serialized. See the repository README for configuration, build, and API
// examples.
package bergamot
