# review-board

A Claude Code plugin for **GitHub-style code review** of git changes. One shared local
Go server hosts many review sessions; you annotate diffs in the browser and Claude applies
your requested changes.

## Prerequisites
- **Go 1.23+** (builds the server binary on first use)
- **git**, **jq**, **curl**, **uuidgen** (used by the `/review` skill)
- A browser opener (`xdg-open`, `explorer.exe`, or `open`)

## Usage
- `/review [name] [--base <ref>]` — review the current changes. Opens the diff in your
  browser; annotate lines/ranges with 🔴 Request change or 💬 Comment; click **Finish
  Review**; Claude applies the requested changes. Re-run `/review` to re-review.
- `/review-server [start|stop|status]` — manage the shared daemon (auto-started by `/review`).

## Configuration

- **Port** — set the `REVIEW_BOARD_PORT` environment variable (default `7654`). Both `/review` and `/review-server` talk to `http://127.0.0.1:$REVIEW_BOARD_PORT`.
- **Diff base** — pass `--base <ref>` to `/review` (default `HEAD`), e.g. `/review --base main`.
- **Auto-review prompt (opt-in)** — create `.claude/review-board.local.md` containing:
  ```yaml
  autoReview: true
  ```
  When present, the `Stop` hook prints a reminder to run `/review` after Claude finishes editing (and only when the repo has a nonempty diff). Off by default.

> **Planned:** a `serverUrl` setting to point `/review` at a shared server running on another machine. Today the server runs locally on `REVIEW_BOARD_PORT`.

## How it works
`/review` computes `git diff`, pushes it to `reviewd` (`POST /api/sessions`), opens the
browser, and polls `GET /api/sessions/:id/review` until you submit. The server embeds the
web UI (`diff2html` + annotation) in a single binary; sessions persist under
`~/.local/state/review-board/`.

## Development
- Server tests: `cd reviewd && go test ./...`
- Frontend logic tests: `cd reviewd && node --test webtest/*.test.mjs`

Built test-first (TDD). `diff2html` is MIT-licensed and vendored under `reviewd/web/vendor/`.
