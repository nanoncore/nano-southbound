#!/usr/bin/env bash
set -euo pipefail

DOC="docs/omci-wifi-spec.md"

if [[ ! -f "$DOC" ]]; then
  echo "missing $DOC"
  exit 1
fi

require() {
  local pattern="$1"
  local message="$2"
  if command -v rg >/dev/null 2>&1; then
    if ! rg -q --fixed-strings "$pattern" "$DOC"; then
      echo "OMCI docs drift: $message"
      exit 1
    fi
  else
    if ! grep -Fq "$pattern" "$DOC"; then
      echo "OMCI docs drift: $message"
      exit 1
    fi
  fi
}

require 'Version: 1.0.0' 'missing spec version'
require 'WifiManager interface (southbound)' 'missing southbound interface section'
require 'SetWifiConfig(onu, config) -> WifiActionResult' 'missing SetWifiConfig contract'
require 'PARTIAL_APPLY' 'missing partial apply error code'
require 'COMMAND_TIMEOUT' 'missing timeout error code'
require 'Parser-compatible behavior is mandatory.' 'missing simulator/parser parity requirement'

echo "OMCI docs check passed ($DOC)"
