#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 3 ]]; then
	printf 'Usage: %s VERSION BINARY_ARTIFACTS_DIR OUTPUT_DIR\n' "$0" >&2
	exit 2
fi

version="$1"
binaries_dir="$2"
output_dir="$3"
if [[ ! "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
	printf 'error: invalid release version: %s\n' "$version" >&2
	exit 2
fi
version="${version#v}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ ! -d "$binaries_dir" ]]; then
	printf 'error: binary artifacts directory not found: %s\n' "$binaries_dir" >&2
	exit 1
fi
if [[ -e "$output_dir" ]]; then
	printf 'error: output directory already exists: %s\n' "$output_dir" >&2
	exit 1
fi

license_files=()
while IFS= read -r -d '' license_file; do
	license_files+=("$license_file")
done < <(find third_party/bergamot-translator -type f \
	\( -iname 'license*' -o -iname 'notice*' \) -print0)
if [[ "${#license_files[@]}" -eq 0 ]]; then
	printf '%s\n' 'error: no native third-party license or notice files found' >&2
	exit 1
fi

mkdir -p "$output_dir"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/bergamot-release.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

for target in 'linux amd64' 'linux arm64' 'darwin arm64'; do
	read -r goos goarch <<< "$target"
	artifact_dir="$binaries_dir/binary-$goos-$goarch"
	binary="$artifact_dir/bergamot-go"
	if [[ ! -f "$binary" ]]; then
		printf 'error: expected native binary not found: %s\n' "$binary" >&2
		exit 1
	fi

	stage="$work_dir/$goos-$goarch"
	package_dir="$stage/bergamot-go"
	mkdir -p "$package_dir"
	cp "$binary" "$package_dir/bergamot-go"
	chmod 755 "$package_dir/bergamot-go"
	cp README.md LICENSE THIRD_PARTY_NOTICES.md "$package_dir/"
	for license_file in "${license_files[@]}"; do
		destination="$package_dir/$license_file"
		mkdir -p "$(dirname "$destination")"
		cp "$license_file" "$destination"
	done

	tar -czf "$output_dir/bergamot-go_${version}_${goos}_${goarch}.tar.gz" \
		-C "$stage" bergamot-go
done

(
	cd "$output_dir"
	sha256sum bergamot-go_*.tar.gz > checksums.txt
)