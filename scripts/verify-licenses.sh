#!/usr/bin/env bash

set -eo pipefail

echo "Checking for incompatible licenses..."

command -v go-licenses >/dev/null 2>&1 || {
    echo "ERROR: go-licenses not found — run inside casjaysdev/go:latest" >&2
    exit 69
}

echo "Scanning dependencies..."
license_ignores=()
overrides_file=.github/license-overrides.txt
if [[ -f $overrides_file ]]; then
    while read -r module _; do
        [[ -z ${module:-} || $module == \#* ]] && continue
        license_ignores+=(--ignore "$module")
    done < "$overrides_file"
fi
licenses=$(go-licenses csv ./src/... "${license_ignores[@]}")
if [[ -f $overrides_file ]]; then
    overrides=$(awk 'NF >= 2 && $1 !~ /^#/ { print $1 "," $2 }' "$overrides_file")
    [[ -n $overrides ]] && licenses+=$'\n'"$overrides"
fi
if echo "$licenses" | grep -iE 'GPL|AGPL|LGPL'; then
    echo "ERROR: Copyleft license detected!" >&2
    echo "Remove the dependency or find an alternative." >&2
    exit 1
fi

unknown_licenses=$(echo "$licenses" | awk -F, 'tolower($NF) == "unknown" { print $0 }')
if [ -n "$unknown_licenses" ]; then
    echo "ERROR: Unknown or unreviewed dependency licenses detected:" >&2
    echo "$unknown_licenses" >&2
    exit 1
fi
echo "✓ All licenses are compatible"
echo "Generating license report in a temporary directory..."
license_tmp_root=${TMPDIR:-/tmp}/apimgr
mkdir -p "$license_tmp_root"
license_report_dir=$(mktemp -d "$license_tmp_root/airports-XXXXXX")
trap 'rm -rf "$license_report_dir"' EXIT
go-licenses csv ./src/... "${license_ignores[@]}" > "$license_report_dir/licenses.csv"
go-licenses save ./src/... "${license_ignores[@]}" --save_path="$license_report_dir/third_party_licenses"

echo "✓ License report generated in a temporary directory."
