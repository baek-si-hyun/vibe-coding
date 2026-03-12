#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

bash "${SCRIPT_DIR}/bootstrap_env.sh"
"${SCRIPT_DIR}/venv/bin/python" "${SCRIPT_DIR}/batch_krx_lstm_export.py" "$@"
