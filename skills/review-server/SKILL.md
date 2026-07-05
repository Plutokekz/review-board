---
name: review-server
description: Start, stop, or check the shared review-board server. Use when the user runs /review-server or asks to start/stop/status the code-review server.
argument-hint: "[start|stop|status]"
allowed-tools: Bash
---

# review-server

Manage the shared `reviewd` daemon (default `http://localhost:7654`).

**Binary:** built on first use to `~/.cache/review-board/reviewd` from `${CLAUDE_PLUGIN_ROOT}/reviewd`.

Run the requested action (`$ARGUMENTS`, default `status`):

```bash
BIN="$HOME/.cache/review-board/reviewd"
PORT="${REVIEW_BOARD_PORT:-7654}"
URL="http://127.0.0.1:$PORT"

ensure_binary() {
  if [ ! -x "$BIN" ]; then
    mkdir -p "$(dirname "$BIN")"
    ( cd "${CLAUDE_PLUGIN_ROOT}/reviewd" && go build -o "$BIN" . ) || { echo "build failed (need Go)"; exit 1; }
  fi
}
is_up() { curl -sf "$URL/api/sessions" >/dev/null 2>&1; }

case "${1:-status}" in
  start)
    ensure_binary
    if is_up; then echo "already running at $URL"; else
      nohup "$BIN" --port "$PORT" >"$HOME/.cache/review-board/reviewd.log" 2>&1 &
      for i in 1 2 3 4 5; do sleep 0.4; is_up && break; done
      is_up && echo "started at $URL" || { echo "failed to start; see reviewd.log"; exit 1; }
    fi ;;
  stop)
    pkill -f "$BIN --port $PORT" && echo "stopped" || echo "not running" ;;
  status)
    if is_up; then echo "up: $(curl -s "$URL/api/sessions" | grep -o '"id"' | wc -l) session(s) at $URL"; else echo "down"; fi ;;
  *) echo "usage: /review-server [start|stop|status]" ;;
esac
```
