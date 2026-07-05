#!/usr/bin/env bash
# End-to-end smoke test for the review-board browser UI (reviewd/web/app.mjs).
# Builds reviewd, starts it on an isolated port + state dir, pushes a fixture
# session, launches headless Chromium with remote debugging, and runs
# scripts/ui-smoke.mjs against it. Exits non-zero if any assertion fails.
#
# Requires: Go, node 22, and chromium/chromium-browser/google-chrome.
# Usage: bash scripts/ui-smoke.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT="${UI_SMOKE_PORT:-7699}"
DBG="${UI_SMOKE_CDP:-9333}"
STATE="$(mktemp -d)"
BINDIR="$(mktemp -d)"
BIN="$BINDIR/reviewd"
UDD="$(mktemp -d)"
SRV=""

cleanup() {
  [ -n "$SRV" ] && kill "$SRV" 2>/dev/null || true
  pkill -f "remote-debugging-port=$DBG" 2>/dev/null || true
  rm -rf "$STATE" "$BINDIR" "$UDD" 2>/dev/null || true
}
trap cleanup EXIT

echo "building reviewd..."
( cd "$ROOT/reviewd" && go build -o "$BIN" . )

echo "starting reviewd on :$PORT ..."
"$BIN" --port "$PORT" --state-dir "$STATE" >/dev/null 2>&1 &
SRV=$!
for i in $(seq 1 20); do sleep 0.3; curl -sf "http://127.0.0.1:$PORT/api/sessions" >/dev/null 2>&1 && break; done
curl -sf "http://127.0.0.1:$PORT/api/sessions" >/dev/null 2>&1 || { echo "server did not start"; exit 1; }

echo "pushing fixture session..."
DIFF=$'diff --git a/new.py b/new.py\nnew file mode 100644\n--- /dev/null\n+++ b/new.py\n@@ -0,0 +1,3 @@\n+def a():\n+    return 1\n+CONST = 2\n'
jq -n --arg id ui-smoke --arg title smoke --arg base HEAD --arg u t0 --arg diff "$DIFF" \
  '{id:$id,title:$title,branch:"main",base:$base,diff:$diff,createdAt:$u,updatedAt:$u}' \
  | curl -sf -X POST -H 'Content-Type: application/json' --data-binary @- "http://127.0.0.1:$PORT/api/sessions" >/dev/null

BROWSER=""
for c in chromium chromium-browser google-chrome google-chrome-stable chrome; do
  command -v "$c" >/dev/null 2>&1 && { BROWSER="$c"; break; }
done
[ -n "$BROWSER" ] || { echo "SKIP: no chromium/chrome found"; exit 0; }

echo "launching $BROWSER (headless, CDP :$DBG) ..."
"$BROWSER" --headless --disable-gpu --no-sandbox --disable-dev-shm-usage \
  --user-data-dir="$UDD" --remote-debugging-port="$DBG" --window-size=1400,1400 \
  about:blank >/dev/null 2>&1 &
for i in $(seq 1 40); do sleep 0.3; curl -sf "http://127.0.0.1:$DBG/json/version" >/dev/null 2>&1 && break; done
curl -sf "http://127.0.0.1:$DBG/json/version" >/dev/null 2>&1 || { echo "CDP did not come up"; exit 1; }

CDP_PORT="$DBG" UI_URL="http://127.0.0.1:$PORT/s/ui-smoke" node "$ROOT/scripts/ui-smoke.mjs"
