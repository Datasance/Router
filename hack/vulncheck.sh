#!/usr/bin/env bash
# Run govulncheck on router module code paths.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

govulncheck -format=text ./...
