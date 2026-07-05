import { anchorFromRow, mergeRange } from '/lib/anchor.mjs';
import { buildReviewPayload } from '/lib/review-payload.mjs';
import { parseDiffFiles } from '/lib/diff-parse.mjs';

const $ = (sel) => document.querySelector(sel);

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error(`${path} -> ${res.status}`);
  return res.status === 204 ? null : res.json();
}

function route() {
  const m = location.pathname.match(/^\/s\/(.+)$/);
  if (m) renderReview(decodeURIComponent(m[1]));
  else renderDashboard();
}

async function renderDashboard() {
  $('#review').classList.add('hidden');
  const el = $('#dashboard');
  el.classList.remove('hidden');
  $('#subtitle').textContent = 'pending reviews';
  const sessions = await api('/api/sessions');
  el.innerHTML = sessions.length === 0
    ? '<p>No pending reviews. Run <code>/review</code> in a repo.</p>'
    : sessions.map((s) => `
      <a class="session-row" href="/s/${encodeURIComponent(s.id)}">
        <strong>${escapeHtml(s.title || s.id)}</strong>
        <span class="badge">${s.status}</span>
        <span class="add">+${s.stats.additions}</span>
        <span class="del">-${s.stats.deletions}</span>
        <span class="badge">${s.stats.files} files</span>
      </a>`).join('');
}

let annotations = [];
let pendingAnchor = null;

async function renderReview(id) {
  $('#dashboard').classList.add('hidden');
  const el = $('#review');
  el.classList.remove('hidden');
  annotations = [];
  pendingAnchor = null;
  const sess = await api(`/api/sessions/${encodeURIComponent(id)}`);
  $('#subtitle').textContent = `${sess.title} — ${sess.branch}`;

  // Our own changed-files summary (uses parseDiffFiles; diff2html's own list is off).
  const filesBar = parseDiffFiles(sess.diff).map((f) =>
    `<span class="badge">${escapeHtml(f.file)} <span class="add">+${f.additions}</span> <span class="del">-${f.deletions}</span></span>`
  ).join(' ');

  el.innerHTML = `
    <div class="toolbar">
      <button id="unified" class="primary">Unified</button>
      <button id="split">Side-by-side</button>
      <button id="refresh">Refresh</button>
      <span style="flex:1"></span>
      <button id="finish" class="primary">Finish Review</button>
    </div>
    <div id="files">${filesBar}</div>
    <textarea id="summary" placeholder="Overall summary (optional)"></textarea>
    <div id="diff"></div>`;

  renderDiff(sess.diff, 'line-by-line');
  $('#refresh').onclick = () => renderReview(id);
  $('#unified').onclick = () => renderDiff(sess.diff, 'line-by-line');
  $('#split').onclick = () => renderDiff(sess.diff, 'side-by-side');
  $('#finish').onclick = () => finish(id);
}

function renderDiff(diff, format) {
  $('#diff').innerHTML = Diff2Html.html(diff, { drawFileList: false, matching: 'lines', outputFormat: format });
  wireDiff();
}

// Extract {file, side, line} from a clicked diff2html code row (v3 DOM;
// handles both line-by-line and side-by-side). Adjust selectors if your
// vendored diff2html renders different markup.
function rowMeta(tr) {
  const wrapper = tr.closest('.d2h-file-wrapper');
  const file = wrapper?.querySelector('.d2h-file-name')?.textContent?.trim() || '';
  let newNum = tr.querySelector('.line-num2')?.textContent?.trim();
  let oldNum = tr.querySelector('.line-num1')?.textContent?.trim();
  if (!newNum && !oldNum) {
    const sideNum = tr.querySelector('.d2h-code-side-linenumber')?.textContent?.trim();
    if (sideNum) {
      const side = tr.closest('.d2h-file-side-diff');
      if (side && side.previousElementSibling !== null) newNum = sideNum;
      else oldNum = sideNum;
    }
  }
  if (newNum && /^\d+$/.test(newNum)) return { file, side: 'new', line: Number(newNum) };
  if (oldNum && /^\d+$/.test(oldNum)) return { file, side: 'old', line: Number(oldNum) };
  return null;
}

function wireDiff() {
  document.querySelectorAll('#diff tr').forEach((tr) => {
    if (!tr.querySelector('.d2h-code-line') && !tr.querySelector('.d2h-code-side-line')) return;
    tr.addEventListener('click', (ev) => {
      const meta = rowMeta(tr);
      if (!meta) return;
      const anchor = anchorFromRow(meta);
      // Shift-click a second line in the same file+side extends into a range.
      if (ev.shiftKey && pendingAnchor &&
          pendingAnchor.file === anchor.file && pendingAnchor.side === anchor.side) {
        openCommentBox(tr, mergeRange(pendingAnchor, anchor));
        pendingAnchor = null;
      } else {
        pendingAnchor = anchor;
        openCommentBox(tr, anchor);
      }
    });
  });
}

function openCommentBox(tr, anchor) {
  const range = anchor.startLine === anchor.endLine
    ? `${anchor.startLine}` : `${anchor.startLine}-${anchor.endLine}`;
  const box = document.createElement('div');
  box.className = 'comment-box';
  box.innerHTML = `
    <div><code>${escapeHtml(anchor.file)}:${range}</code> (${anchor.side}) — shift-click another line for a range</div>
    <label><input type="radio" name="t" value="request_change" checked> 🔴 Request change</label>
    <label><input type="radio" name="t" value="comment"> 💬 Comment</label>
    <textarea placeholder="Your note…"></textarea>
    <button class="save primary">Save</button>`;
  box.querySelector('.save').onclick = () => {
    annotations.push({
      anchor,
      type: box.querySelector('input[name=t]:checked').value,
      body: box.querySelector('textarea').value,
    });
    const ta = box.querySelector('textarea');
    ta.disabled = true;
    const btn = box.querySelector('.save');
    btn.textContent = 'Saved ✓';
    btn.disabled = true;
  };
  tr.after(box);
}

async function finish(id) {
  const nowIso = new Date().toISOString();
  const payload = buildReviewPayload(annotations, $('#summary').value, nowIso);
  await api(`/api/sessions/${encodeURIComponent(id)}/review`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  document.querySelector('main').innerHTML =
    '<p>✅ Review submitted. You can close this tab — Claude is applying your changes.</p>';
}

function escapeHtml(s) {
  return s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

window.addEventListener('popstate', route);
route();
