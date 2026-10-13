#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$task_root"
make build
export GENCO_DATA_DIR="$task_root/data"
export GENCO_FRONTEND_DIR="$task_root/frontend/dist"
exec "$task_root/bin/genco-finance"
