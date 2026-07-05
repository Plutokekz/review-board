---
name: review-server
description: Start, stop, or check the shared review-board server. Use when the user runs /review-server or asks to start/stop/status the code-review server.
argument-hint: "[start|stop|status]"
allowed-tools: Bash
---

# review-server

Manage the shared `reviewd` daemon (default `http://localhost:7654`).

**Binary:** on first use, downloaded from GitHub Releases for your platform to `~/.cache/review-board/reviewd` (or built from `${CLAUDE_PLUGIN_ROOT}/reviewd` with Go if no prebuilt binary matches).

Determine the requested action from the user's input — `start`, `stop`, or `status` (default `status`) — and set ACTION in the script below before running it.

```bash
ACTION="status"        # ← replace with the requested action: start | stop | status
BIN="$HOME/.cache/review-board/reviewd"
PORT="${REVIEW_BOARD_PORT:-7654}"
URL="http://127.0.0.1:$PORT"

ensure_binary() {
  [ -x "$BIN" ] && return 0
  mkdir -p "$(dirname "$BIN")"
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m); case "$ARCH" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
  # 1) prefer a prebuilt release binary (no Go needed)
  curl -fsSL "https://github.com/Plutokekz/review-board/releases/latest/download/reviewd-${OS}-${ARCH}" -o "$BIN" 2>/dev/null && chmod +x "$BIN"
  # 2) fall back to building from source (needs Go)
  if [ ! -x "$BIN" ] && command -v go >/dev/null 2>&1; then
    ( cd "${CLAUDE_PLUGIN_ROOT}/reviewd" && go build -o "$BIN" . )
  fi
  [ -x "$BIN" ] || { echo "could not obtain reviewd: no prebuilt binary for ${OS}/${ARCH} and Go is not installed"; exit 1; }
}
is_up() { curl -sf "$URL/api/sessions" >/dev/null 2>&1; }

case "$ACTION" in
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
    if is_up; then echo "up: $(curl -s "$URL/api/sessions" | jq 'length') session(s) at $URL"; else echo "down"; fi ;;
  *) echo "usage: /review-server [start|stop|status]"; exit 1 ;;
esac
```
