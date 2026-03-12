#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${SCRIPT_DIR}/venv"
REQ_FILE="${SCRIPT_DIR}/requirements.txt"
TARGET_PYTHON_MINOR="3.12"

has_target_python() {
  local candidate="$1"
  [ -x "${candidate}" ] || return 1
  "${candidate}" - <<'PY'
import sys
raise SystemExit(0 if sys.version_info[:2] == (3, 12) else 1)
PY
}

find_target_python() {
  local candidate

  for candidate in \
    "${VENV_DIR}/bin/python" \
    "$(command -v python3.12 2>/dev/null || true)" \
    "/opt/homebrew/bin/python3.12" \
    "/opt/homebrew/opt/python@3.12/bin/python3.12" \
    "/usr/local/bin/python3.12"
  do
    if [ -n "${candidate}" ] && has_target_python "${candidate}"; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  if command -v brew >/dev/null 2>&1; then
    echo "Python ${TARGET_PYTHON_MINOR} not found. Installing with Homebrew..." >&2
    HOMEBREW_NO_AUTO_UPDATE=1 brew install python@3.12 1>&2
    if has_target_python "/opt/homebrew/bin/python3.12"; then
      printf '%s\n' "/opt/homebrew/bin/python3.12"
      return 0
    fi
    if has_target_python "/opt/homebrew/opt/python@3.12/bin/python3.12"; then
      printf '%s\n' "/opt/homebrew/opt/python@3.12/bin/python3.12"
      return 0
    fi
    if has_target_python "/usr/local/bin/python3.12"; then
      printf '%s\n' "/usr/local/bin/python3.12"
      return 0
    fi
  fi

  echo "Unable to locate a compatible Python ${TARGET_PYTHON_MINOR} interpreter." >&2
  exit 1
}

ensure_venv() {
  local python_bin="$1"
  local rebuild=0

  if [ ! -x "${VENV_DIR}/bin/python" ]; then
    rebuild=1
  elif ! has_target_python "${VENV_DIR}/bin/python"; then
    rebuild=1
  fi

  if [ "${rebuild}" -eq 1 ]; then
    echo "Creating Python ${TARGET_PYTHON_MINOR} virtual environment..."
    rm -rf "${VENV_DIR}"
    "${python_bin}" -m venv "${VENV_DIR}"
  fi
}

dependencies_ready() {
  "${VENV_DIR}/bin/python" - <<'PY'
from importlib.util import find_spec

modules = ["numpy", "pandas", "sklearn", "matplotlib", "tensorflow"]
for name in modules:
    if find_spec(name) is None:
        raise SystemExit(1)
PY
}

ensure_requirements() {
  local req_hash
  local stamp_file="${VENV_DIR}/.requirements-sha256"
  local current_hash=""

  req_hash="$(shasum -a 256 "${REQ_FILE}" | awk '{print $1}')"
  if [ -f "${stamp_file}" ]; then
    current_hash="$(cat "${stamp_file}")"
  fi

  if [ "${req_hash}" != "${current_hash}" ] || ! dependencies_ready >/dev/null 2>&1; then
    echo "Installing LSTM dependencies..."
    "${VENV_DIR}/bin/python" -m pip install --upgrade pip setuptools wheel
    "${VENV_DIR}/bin/python" -m pip install -r "${REQ_FILE}"
    printf '%s' "${req_hash}" > "${stamp_file}"
  fi
}

main() {
  local python_bin
  local python_version
  python_bin="$(find_target_python)"
  ensure_venv "${python_bin}"
  ensure_requirements
  python_version="$("${VENV_DIR}/bin/python" --version)"
  echo "LSTM environment ready: ${VENV_DIR}"
  echo "Python: ${python_version}"
}

main "$@"
