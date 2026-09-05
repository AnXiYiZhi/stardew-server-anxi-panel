#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
validator="$repo_root/scripts/validate-release-matrix.sh"

expect_success() {
  if ! bash "$validator" "$@"; then
    echo "expected release matrix to pass: $*" >&2
    exit 1
  fi
}

expect_failure() {
  if bash "$validator" "$@" >/dev/null 2>&1; then
    echo "expected release matrix to fail: $*" >&2
    exit 1
  fi
}

expect_success --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.3.2 --require-oldest-for-zero-patch
expect_success --version 0.6.1 --previous-version 0.6.0 --oldest-version 0.3.2
expect_success --version 0.7.0 --previous-version 0.6.1 --require-oldest-for-zero-patch
expect_failure --version 0.7.0 --previous-version 0.6.1 --oldest-version 0.3.2
expect_failure --version 0.7.0 --previous-version 0.6.1 --oldest-version 0.6.0
expect_failure --version 0.7.0 --previous-version 0.6.0 --oldest-version 0.3.2
expect_success --version 0.5.14 --previous-version 0.5.13 --require-oldest-for-zero-patch
expect_success --version 1.2.3 --previous-version 1.2.2 --oldest-version 1.0.0

expect_failure --version 0.6.0 --previous-version 0.5.13 --require-oldest-for-zero-patch
expect_failure --version 0.6.1 --previous-version 0.6.0
expect_failure --version 0.6.1 --previous-version 0.5.13 --oldest-version 0.3.2
expect_failure --version 0.6.1 --previous-version 0.6.0 --oldest-version 0.4.0
expect_failure --version 0.6.0 --previous-version 0.6.0 --oldest-version 0.3.2
expect_failure --version 0.5.13 --previous-version 0.6.0 --oldest-version 0.3.2
expect_failure --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.5.13
expect_failure --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.5.14
expect_failure --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.6.0
expect_failure --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.03.2
expect_failure --version 0.6.0 --previous-version 0.5.13 --oldest-version 0.4.0 --require-oldest-for-zero-patch
expect_failure --version 0.6.0 --previous-version 0.5.12 --oldest-version 0.3.2 --require-oldest-for-zero-patch

echo "release matrix tests passed"
