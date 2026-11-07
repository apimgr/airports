#!/usr/bin/env bash

set -eo pipefail

license_output_dir=${LICENSE_OUTPUT_DIR:-.}

echo "Checking for incompatible licenses..."

command -v go-licenses >/dev/null 2>&1 || {
    echo "ERROR: go-licenses not found — run inside casjaysdev/go:latest" >&2
    exit 1
}

echo "Scanning dependencies..."
license_go_flags="${GOFLAGS:+$GOFLAGS }-tags=gui"
licenses=$(GOFLAGS="$license_go_flags" go-licenses csv --ignore=modernc.org/mathutil ./...)
if echo "$licenses" | grep -iE 'GPL|AGPL|LGPL'; then
    echo "ERROR: Copyleft license detected!" >&2
    echo "Remove the dependency or find an alternative." >&2
    exit 1
fi

unknown_licenses=$(echo "$licenses" | awk -F, 'tolower($3) == "unknown" { print $1 "," $2 "," $3 }')
if [ -n "$unknown_licenses" ]; then
    echo "ERROR: Unknown or unreviewed dependency licenses detected:" >&2
    echo "$unknown_licenses" >&2
    exit 1
fi
mathutil_version=$(go list -m -f '{{.Version}}' modernc.org/mathutil)
if [ "$mathutil_version" != 'v1.7.1' ]; then
    echo "ERROR: modernc.org/mathutil version changed to $mathutil_version; re-review its upstream license" >&2
    exit 1
fi

echo "✓ All licenses are compatible"
echo "Generating license report..."
mkdir -p "$license_output_dir"
GOFLAGS="$license_go_flags" go-licenses csv --ignore=modernc.org/mathutil ./... > "$license_output_dir/licenses.csv"
printf '%s\n' 'modernc.org/mathutil,https://gitlab.com/cznic/mathutil/-/blob/master/LICENSE,BSD-3-Clause' >> "$license_output_dir/licenses.csv"
GOFLAGS="$license_go_flags" go-licenses save --ignore=modernc.org/mathutil ./... --save_path="$license_output_dir/third_party_licenses"

echo "✓ License report saved to $license_output_dir/licenses.csv and $license_output_dir/third_party_licenses/"
echo ""
echo "Next steps:"
echo "1. Review licenses.csv"
echo "2. Update LICENSE.md with any new dependencies"
echo "3. Commit the changes"
