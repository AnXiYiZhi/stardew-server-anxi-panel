#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 --version <x.y.z> --previous-version <x.y.z> [--oldest-version <x.y.z>] [--require-oldest-for-zero-patch]" >&2
}

version=""
previous_version=""
oldest_version=""
require_oldest_for_zero_patch=0
while (($# > 0)); do
  case "$1" in
    --version)
      version="${2:-}"
      shift 2
      ;;
    --previous-version)
      previous_version="${2:-}"
      shift 2
      ;;
    --oldest-version)
      oldest_version="${2:-}"
      shift 2
      ;;
    --require-oldest-for-zero-patch)
      require_oldest_for_zero_patch=1
      shift
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done

semver_pattern='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'
if [[ ! "$version" =~ $semver_pattern || ! "$previous_version" =~ $semver_pattern ]]; then
  echo "release matrix: version and previous version must be exact stable semantic versions" >&2
  exit 1
fi

if [[ "$version" == "$previous_version" || "$(printf '%s\n%s\n' "$previous_version" "$version" | sort -V | tail -n 1)" != "$version" ]]; then
  echo "release matrix: candidate version must be newer than previous version" >&2
  exit 1
fi

if [[ -n "$oldest_version" ]]; then
  if [[ ! "$oldest_version" =~ $semver_pattern ]]; then
    echo "release matrix: oldest version must be an exact stable semantic version" >&2
    exit 1
  fi
  if [[ "$oldest_version" == "$previous_version" || "$(printf '%s\n%s\n' "$oldest_version" "$previous_version" | sort -V | tail -n 1)" != "$previous_version" ]]; then
    echo "release matrix: oldest version must be strictly older than previous version" >&2
    exit 1
  fi
fi

# Release-specific upgrade contracts are intentionally version-controlled.
# They prevent a syntactically valid but incomplete proof from weakening a
# matrix that was declared mandatory for a compatibility-changing release.
case "$version" in
  0.7.0)
    if [[ "$previous_version" != "0.6.1" || -n "$oldest_version" ]]; then
      echo "release matrix: 0.7.0 requires the approved 0.6.1 upgrade source" >&2
      exit 1
    fi
    ;;
  0.6.0)
    if [[ "$previous_version" != "0.5.13" || "$oldest_version" != "0.3.2" ]]; then
      echo "release matrix: 0.6.0 requires previous 0.5.13 and affected oldest 0.3.2" >&2
      exit 1
    fi
    ;;
  0.6.1)
    if [[ "$previous_version" != "0.6.0" || "$oldest_version" != "0.3.2" ]]; then
      echo "release matrix: 0.6.1 requires previous 0.6.0 and affected oldest 0.3.2" >&2
      exit 1
    fi
    ;;
esac

IFS=. read -r _ _ version_patch <<<"$version"
if ((require_oldest_for_zero_patch == 1)) && [[ "$version_patch" == 0 && -z "$oldest_version" && "$version" != 0.7.0 ]]; then
  echo "release matrix: an affected oldest version is required for an explicit zero-patch release" >&2
  exit 1
fi
