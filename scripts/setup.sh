#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

JOBS="${JOBS:-2}"

usage() {
	cat <<'EOF'
Usage: ./scripts/setup.sh

Initializes pinned recursive submodules, applies the local compatibility patches
when needed, builds Bergamot's CPU library under build/bergamot, and verifies the
cgo-linked Go packages compile. The translation model for TestBridgeSmoke is
separate; see README.md to download it.

Set JOBS to change native build parallelism (default: 2).
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
	usage
	exit 0
elif [[ "$#" -gt 0 ]]; then
	usage >&2
	exit 2
fi

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		printf 'error: required command not found: %s\n' "$1" >&2
		exit 1
	fi
}

for command in git cmake make go cc; do
	require_command "$command"
done
if ! command -v "${CXX:-c++}" >/dev/null 2>&1; then
	printf 'error: C++ compiler not found (%s); install a C++17 toolchain\n' "${CXX:-c++}" >&2
	exit 1
fi

case "$(uname -s)-$(uname -m)" in
	Darwin-arm64)
		;;
	Linux-x86_64|Linux-aarch64)
		printf 'warning: Linux cgo flags are present but this target has not been verified end-to-end\n' >&2
		;;
	*)
		printf 'warning: %s/%s is not a verified target; cgo flags and native dependencies may need adaptation\n' "$(uname -s)" "$(uname -m)" >&2
		;;
esac

printf '%s\n' 'Initializing pinned Bergamot submodules...'
git submodule update --init --recursive third_party/bergamot-translator

apply_patch_once() {
	local repository="$1"
	local patch_file="$2"
	if git -C "$repository" apply --check "$patch_file" 2>/dev/null; then
		git -C "$repository" apply "$patch_file"
		printf 'Applied %s\n' "$patch_file"
	elif git -C "$repository" apply --reverse --check "$patch_file" 2>/dev/null; then
		printf 'Already applied: %s\n' "$patch_file"
	else
		printf 'error: patch does not apply to %s; verify the pinned submodule revision and review %s\n' "$repository" "$patch_file" >&2
		exit 1
	fi
}

MARIAN=third_party/bergamot-translator/3rd_party/marian-dev
SENTENCEPIECE="$MARIAN/src/3rd_party/sentencepiece"
apply_patch_once "$MARIAN" "$ROOT/patches/marian-macos-arm64.patch"
apply_patch_once "$MARIAN" "$ROOT/patches/marian-arm64-simd.patch"
apply_patch_once "$SENTENCEPIECE" "$ROOT/patches/sentencepiece-macos-arm64.patch"

printf '%s\n' 'Configuring and building the Bergamot CPU library...'
cmake -S third_party/bergamot-translator -B build/bergamot -DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_POLICY_VERSION_MINIMUM=3.5
cmake --build build/bergamot --target bergamot-translator --parallel "$JOBS"

printf '%s\n' 'Building the cgo-linked Go packages...'
CGO_ENABLED=1 go build ./...

printf '%s\n' 'Setup complete. See README.md for model download and smoke-test instructions.'