# Third-Party Notices

The MIT license in the repository-root `LICENSE` applies to original
`bergamot-go` project files only. It does not change the license of any
third-party component, including code in `third_party/bergamot-translator/`
or third-party code incorporated into other files. In particular, local
patches to upstream code remain subject to the applicable upstream terms when
applied.

This file records third-party components identified in the current Go module
and recursive Bergamot source tree. Keep the upstream license files with those
components. When distributing a source archive or binary that includes a
component, include the applicable license text and notices listed below.

## Go modules

| Component | Version | License | License text / source |
| --- | --- | --- | --- |
| `gopkg.in/yaml.v3` | `v3.0.1` | MIT OR Apache-2.0 | [Upstream source](https://github.com/go-yaml/yaml/tree/v3.0.1); retain the license files supplied with the module. |
| `gopkg.in/check.v1` | `v0.0.0-20161208181325-20d25e280405` | BSD-2-Clause | [Upstream source](https://github.com/go-check/check); test dependency recorded in `go.sum`. |

## Native source dependencies

The native dependencies below are fetched as recursive submodules under
`third_party/bergamot-translator/`. Their upstream license files are retained
at the paths shown.

| Component | License | License text / scope |
| --- | --- | --- |
| Bergamot Translator | MPL-2.0 | [`third_party/bergamot-translator/LICENSE`](third_party/bergamot-translator/LICENSE) |
| Marian | MIT | [`third_party/bergamot-translator/3rd_party/marian-dev/LICENSE.md`](third_party/bergamot-translator/3rd_party/marian-dev/LICENSE.md) |
| `ssplit-cpp` source and build files | Apache-2.0 | [`third_party/bergamot-translator/3rd_party/ssplit-cpp/LICENSE.md`](third_party/bergamot-translator/3rd_party/ssplit-cpp/LICENSE.md) identifies the license and copyright notice. The full license is at <https://www.apache.org/licenses/LICENSE-2.0>. |
| `ssplit-cpp` `nonbreaking_prefixes` data | LGPL-2.1 | The same `ssplit-cpp/LICENSE.md` identifies this data's origin and terms. The full license is at <https://www.gnu.org/licenses/old-licenses/lgpl-2.1.txt>. The upstream notice says these files are read at runtime, not compiled into the library. |
| SentencePiece | Apache-2.0 | [`third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/sentencepiece/LICENSE`](third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/sentencepiece/LICENSE) |
| `yaml-cpp` | MIT | [`third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/yaml-cpp/LICENSE`](third_party/bergamot-translator/3rd_party/marian-dev/src/3rd_party/yaml-cpp/LICENSE) |
| `pybind11` | BSD-3-Clause | [`third_party/bergamot-translator/3rd_party/pybind11/LICENSE`](third_party/bergamot-translator/3rd_party/pybind11/LICENSE). The project README notes that it is not needed for the default native library target. |

This inventory reflects the dependencies identified in the current module
files and recursive native source tree; it is not a complete software bill of
materials for every possible build or release. Audit the exact artifacts,
runtime data, and system libraries included in each distribution, and include
the corresponding full license texts and notices with that distribution.