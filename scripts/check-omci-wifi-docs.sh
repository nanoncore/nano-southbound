#!/usr/bin/env bash
set -euo pipefail

DOC="docs/omci-wifi-spec.md"
EXPECTED_HASH="c43f5d2b1d581152d73dade303c3a8d3c1f117734699a9208b73fa604795aff5"

if [[ ! -f "$DOC" ]]; then
  echo "missing $DOC"
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_HASH="$(sha256sum "$DOC" | awk '{print $1}')"
else
  ACTUAL_HASH="$(shasum -a 256 "$DOC" | awk '{print $1}')"
fi
if [[ "$ACTUAL_HASH" != "$EXPECTED_HASH" ]]; then
  echo "OMCI docs drift: $DOC hash mismatch"
  echo "expected: $EXPECTED_HASH"
  echo "actual:   $ACTUAL_HASH"
  echo "canonical: nanoncore/docs/omci-wifi-spec.md"
  exit 1
fi

echo "OMCI docs check passed ($DOC, sha256=$ACTUAL_HASH)"
