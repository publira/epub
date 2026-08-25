#!/usr/bin/env bash

set -euo pipefail

# renovate: datasource=github-releases depName=w3c/epubcheck versioning=semver extractVersion=^v(?<version>.*)$
EPUBCHECK_VERSION="5.3.0"

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tools_dir="${repo_dir}/.tools"
epubcheck_dir="${tools_dir}/epubcheck-${EPUBCHECK_VERSION}"
jar_path="${epubcheck_dir}/epubcheck.jar"

if ! command -v java >/dev/null 2>&1; then
  echo "error: Java 11 or newer is required to run EPUBCheck" >&2
  exit 1
fi

if [[ ! -f "${jar_path}" ]]; then
  for command_name in curl unzip; do
    if ! command -v "${command_name}" >/dev/null 2>&1; then
      echo "error: ${command_name} is required to install EPUBCheck" >&2
      exit 1
    fi
  done

  echo "Downloading EPUBCheck v${EPUBCHECK_VERSION}..." >&2
  mkdir -p "${tools_dir}"
  temp_dir="$(mktemp -d "${tools_dir}/epubcheck-download.XXXXXX")"
  trap 'rm -rf "${temp_dir}"' EXIT
  archive_path="${temp_dir}/epubcheck.zip"

  curl --fail --location --silent --show-error \
    --output "${archive_path}" \
    "https://github.com/w3c/epubcheck/releases/download/v${EPUBCHECK_VERSION}/epubcheck-${EPUBCHECK_VERSION}.zip"
  unzip -q "${archive_path}" -d "${temp_dir}"
  if [[ ! -f "${temp_dir}/epubcheck-${EPUBCHECK_VERSION}/epubcheck.jar" ]]; then
    echo "error: downloaded EPUBCheck archive does not contain epubcheck.jar" >&2
    exit 1
  fi
  mv "${temp_dir}/epubcheck-${EPUBCHECK_VERSION}" "${epubcheck_dir}"
fi

exec java -jar "${jar_path}" "$@"
