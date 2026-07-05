---
name: review
description: Open a GitHub-style browser review of the current git changes, then apply the reviewer's requested changes. Use when the user runs /review or asks to review the current changes / diff before continuing.
argument-hint: "[review-name] [--base <ref>]"
allowed-tools: Bash, Read, Edit, Write
---

# review

Push the current git diff to the shared review server, open the browser, wait for the
reviewer to submit, then apply every requested change.

## 1. Push the diff and open the browser

Parse `$ARGUMENTS`: an optional review name (first non-flag token) and an optional
`--base <ref>` (default `HEAD`). Then run:

```bash
BIN="$HOME/.cache/review-board/reviewd"
PORT="${REVIEW_BOARD_PORT:-7654}"
URL="http://127.0.0.1:$PORT"
BASE="HEAD"          # override from --base
NAME=""              # override from first positional arg
GIT="${REVIEW_BOARD_GIT:-git}"   # override (e.g. /usr/bin/git) to dodge a shell alias/function named `git`

# --- ensure repo + changes ---
command "$GIT" rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "not a git repo"; exit 1; }
command "$GIT" add -N -- :/ 2>/dev/null || true    # intent-to-add so NEW files show up in the diff
if command "$GIT" diff --no-ext-diff --quiet "$BASE"; then echo "no changes vs $BASE — nothing to review"; exit 0; fi

# --- ensure server (build on first use, start if down) ---
if [ ! -x "$BIN" ]; then
  mkdir -p "$(dirname "$BIN")"
  OS=$(uname -s | tr '[:upper:]' '[:lower:]'); ARCH=$(uname -m); case "$ARCH" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac
  curl -fsSL "https://github.com/Plutokekz/review-board/releases/latest/download/reviewd-${OS}-${ARCH}" -o "$BIN" 2>/dev/null && chmod +x "$BIN"
  if [ ! -x "$BIN" ] && command -v go >/dev/null 2>&1; then ( cd "${CLAUDE_PLUGIN_ROOT}/reviewd" && go build -o "$BIN" . ); fi
  [ -x "$BIN" ] || { echo "could not obtain reviewd (no prebuilt binary for $OS/$ARCH, and Go not installed)"; exit 1; }
fi
curl -sf "$URL/api/sessions" >/dev/null 2>&1 || { nohup "$BIN" --port "$PORT" >"$HOME/.cache/review-board/reviewd.log" 2>&1 & for i in 1 2 3 4 5; do sleep 0.4; curl -sf "$URL/api/sessions" >/dev/null 2>&1 && break; done; }

# --- identity ---
REPO_ROOT="$(command "$GIT" rev-parse --show-toplevel)"
BRANCH="$(command "$GIT" branch --show-current)"
ID="${NAME:-${CLAUDE_CODE_SESSION_ID:-$(uuidgen)}}"
IDENC="$(jq -rn --arg s "$ID" '$s|@uri')"    # percent-encode id for use in URL paths
TITLE="${NAME:-$(basename "$REPO_ROOT")} · ${BRANCH:-detached}"
TS="$(date -Iseconds)"

# --- push (jq encodes the diff safely) ---
DIFF_TMP="$(mktemp)"
command "$GIT" diff --no-ext-diff --no-color "$BASE" > "$DIFF_TMP"

# Guard: a git wrapper/hook (rtk, difftastic, delta, or a `git` alias) can rewrite
# `git diff` into a summary the reviewer UI cannot parse. Catch that here instead of
# silently pushing a non-patch that renders as an empty review. A real unified diff
# always has a `diff --git ` header per changed file.
if [ -s "$DIFF_TMP" ] && ! grep -q '^diff --git ' "$DIFF_TMP"; then
  echo "error: 'git diff' did not produce a unified patch (first line: $(head -1 "$DIFF_TMP"))." >&2
  echo "A git wrapper/hook is likely rewriting its output. Re-run with" >&2
  echo "  REVIEW_BOARD_GIT=/usr/bin/git   (or exclude 'git diff' from the wrapper)." >&2
  rm -f "$DIFF_TMP"; exit 1
fi
jq -n --arg id "$ID" --arg title "$TITLE" --arg repo "$REPO_ROOT" \
      --arg branch "$BRANCH" --arg base "$BASE" --arg createdAt "$TS" --arg updatedAt "$TS" \
      --rawfile diff "$DIFF_TMP" \
      '{id:$id,title:$title,repo:$repo,branch:$branch,base:$base,diff:$diff,createdAt:$createdAt,updatedAt:$updatedAt}' \
  | curl -sf -X POST -H 'Content-Type: application/json' --data-binary @- "$URL/api/sessions" >/dev/null || { echo "push failed — is the server reachable at $URL ?"; rm -f "$DIFF_TMP"; exit 1; }
rm -f "$DIFF_TMP"

REVIEW_URL="$URL/s/$IDENC"
( xdg-open "$REVIEW_URL" >/dev/null 2>&1 || explorer.exe "$REVIEW_URL" >/dev/null 2>&1 || open "$REVIEW_URL" >/dev/null 2>&1 ) &
echo "Review open at $REVIEW_URL — annotate lines, then click Finish Review."
echo "SESSION_ID=$ID"
```

Tell the user the review URL and that you are waiting for them to click **Finish Review**.

## 2. Wait for the submitted review

Poll every ~3s for up to ~20 minutes (400 iterations). Substitute the `SESSION_ID`
printed above:

```bash
URL="http://127.0.0.1:${REVIEW_BOARD_PORT:-7654}"
ID="<SESSION_ID>"
IDENC="$(jq -rn --arg s "$ID" '$s|@uri')"    # percent-encode id for use in URL paths
for i in $(seq 1 400); do
  STATUS="$(curl -sf "$URL/api/sessions/$IDENC/review" | jq -r '.status')"
  [ "$STATUS" = "submitted" ] && break
  sleep 3
done
curl -sf "$URL/api/sessions/$IDENC/review" | jq .
```

If it never becomes `submitted`, tell the user the submission is durable — they can re-run
`/review` later to pick it up — and stop.

## 3. Apply the review

From the submitted JSON, work through each entry in `comments` (all are plain review
comments, GitHub-style — you read each one and act on it):
- Locate `file` at lines `startLine`–`endLine` (the `side` tells you whether the anchor is
  on the new or old version of the file), read `body`, and do what it asks — make the change
  it requests, or if it's a question, answer it and make any change it implies.
- Honour the overall `summary` as top-level guidance.

After applying, give the user a concise list of what you changed per comment, then offer to
**re-review**: re-running `/review` re-pushes the updated diff to the same id so they can
check your work.
