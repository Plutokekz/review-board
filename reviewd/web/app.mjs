import { anchorFromRow, mergeRange } from '/lib/anchor.mjs';
import { buildReviewPayload } from '/lib/review-payload.mjs';
import { parseDiffFiles } from '/lib/diff-parse.mjs';

const $ = (sel) => document.querySelector(sel);

const THEMES = ['auto', 'light', 'dark'];
let theme = 'auto';
try { theme = localStorage.getItem('rb-theme') || 'auto'; } catch (e) { /* storage may be blocked */ }

function applyTheme() {
  document.documentElement.style.colorScheme = theme === 'auto' ? '' : theme;
  document.querySelectorAll('.d2h-wrapper').forEach((w) => {
    w.classList.remove('d2h-light-color-scheme', 'd2h-dark-color-scheme', 'd2h-auto-color-scheme');
    w.classList.add(`d2h-${theme}-color-scheme`);
  });
  const btn = document.getElementById('theme');
  if (btn) btn.textContent = `Theme: ${theme[0].toUpperCase()}${theme.slice(1)}`;
}

function cycleTheme() {
  theme = THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length];
  try { localStorage.setItem('rb-theme', theme); } catch (e) { /* storage may be blocked */ }
  applyTheme();
}

async function api(path, opts) {
  const res = await fetch(path, opts);
  if (!res.ok) throw new Error(`${path} -> ${res.status}`);
  return res.status === 204 ? null : res.json();
}

function route() {
  const m = location.pathname.match(/^\/s\/(.+)$/);
  const p = m ? renderReview(decodeURIComponent(m[1])) : renderDashboard();
  Promise.resolve(p).catch((err) => {
    document.querySelector('main').innerHTML =
      `<p>Could not load: ${escapeHtml(String(err && err.message || err))}. Try reloading.</p>`;
  });
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
      <div class="session-row">
        <a class="session-link" href="/s/${encodeURIComponent(s.id)}">
          <strong>${escapeHtml(s.title || s.id)}</strong>
          <span class="badge">${escapeHtml(s.status)}</span>
          <span class="add">+${s.stats.additions}</span>
          <span class="del">-${s.stats.deletions}</span>
          <span class="badge">${s.stats.files} files</span>
        </a>
        <button class="dismiss" data-id="${escapeHtml(s.id)}" title="Dismiss">&times;</button>
      </div>`).join('');
  el.querySelectorAll('.dismiss').forEach((b) => {
    b.addEventListener('click', async (ev) => {
      ev.preventDefault();
      ev.stopPropagation();
      await api(`/api/sessions/${encodeURIComponent(b.dataset.id)}`, { method: 'DELETE' });
      renderDashboard();
    });
  });
}

let annotations = [];
let sel = null;      // active selection: { file, side, anchorLine, anchorRow, current }
let openBox = null;  // holder <tr> of the currently-open (unsaved) comment box
let loadedUpdatedAt = null;

function rangeLabel(a) {
  return a.startLine === a.endLine ? `${a.startLine}` : `${a.startLine}-${a.endLine}`;
}

function clearSelection() {
  document.querySelectorAll('#diff tr.rb-selected').forEach((r) => r.classList.remove('rb-selected'));
  if (openBox) { openBox.remove(); openBox = null; }
  sel = null;
}

// Highlight the code rows anchorRow..targetRow (inclusive) within one file.
function highlightRange(anchorRow, targetRow) {
  document.querySelectorAll('#diff tr.rb-selected').forEach((r) => r.classList.remove('rb-selected'));
  const wrapper = anchorRow.closest('.d2h-file-wrapper');
  if (!wrapper || targetRow.closest('.d2h-file-wrapper') !== wrapper) {
    anchorRow.classList.add('rb-selected');
    return;
  }
  const rows = Array.from(wrapper.querySelectorAll('tr'));
  let a = rows.indexOf(anchorRow), b = rows.indexOf(targetRow);
  if (a > b) [a, b] = [b, a];
  for (let i = a; i <= b; i++) {
    const r = rows[i];
    if (r.querySelector('.d2h-code-line') || r.querySelector('.d2h-code-side-line')) r.classList.add('rb-selected');
  }
}

async function renderReview(id) {
  $('#dashboard').classList.add('hidden');
  const el = $('#review');
  el.classList.remove('hidden');
  annotations = [];
  sel = null;
  openBox = null;
  const sess = await api(`/api/sessions/${encodeURIComponent(id)}`);
  loadedUpdatedAt = sess.updatedAt;
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
      <button id="approve">Approve</button>
      <button id="request" class="primary">Request changes</button>
    </div>
    <div id="files">${filesBar}</div>
    <textarea id="summary" placeholder="Overall summary (optional)"></textarea>
    <div id="diff"></div>`;

  renderDiff(sess.diff, 'line-by-line');
  $('#refresh').onclick = () => renderReview(id);
  $('#unified').onclick = () => { setActive('unified'); renderDiff(sess.diff, 'line-by-line'); };
  $('#split').onclick = () => { setActive('split'); renderDiff(sess.diff, 'side-by-side'); };
  $('#approve').onclick = () => finish(id, 'approve');
  $('#request').onclick = () => finish(id, 'request-changes');
}

function setActive(which) {
  document.querySelector('#unified').classList.toggle('primary', which === 'unified');
  document.querySelector('#split').classList.toggle('primary', which === 'split');
}

function renderDiff(diff, format) {
  $('#diff').innerHTML = Diff2Html.html(diff, { drawFileList: false, matching: 'lines', outputFormat: format, colorScheme: theme });
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
      // Shift-click extends the current selection; the box stays anchored where it started.
      if (ev.shiftKey && sel && openBox && sel.file === meta.file && sel.side === meta.side) {
        sel.current = mergeRange(
          anchorFromRow({ file: sel.file, side: sel.side, line: sel.anchorLine }),
          anchorFromRow(meta),
        );
        highlightRange(sel.anchorRow, tr);
        const loc = openBox.querySelector('.rb-loc');
        if (loc) loc.textContent = `${sel.current.file}:${rangeLabel(sel.current)} (${sel.current.side})`;
        return;
      }
      // Plain click starts a fresh single-line selection + comment box.
      clearSelection();
      const anchor = anchorFromRow(meta);
      sel = { file: meta.file, side: meta.side, anchorLine: meta.line, anchorRow: tr, current: anchor };
      tr.classList.add('rb-selected');
      openCommentBox(tr, anchor);
    });
  });
}

function openCommentBox(tr, anchor) {
  const box = document.createElement('div');
  box.className = 'comment-box';
  box.innerHTML = `
    <div class="rb-head"><code class="rb-loc">${escapeHtml(anchor.file)}:${rangeLabel(anchor)} (${anchor.side})</code>
      <span class="rb-hint">shift-click another line to select a range</span></div>
    <textarea placeholder="Leave a comment"></textarea>
    <div class="rb-actions"><button class="save primary">Comment</button><button class="cancel">Cancel</button></div>`;
  // Insert as a valid full-width table row, anchored below the FIRST-clicked line
  // so the box never jumps as the selection grows.
  const holderRow = document.createElement('tr');
  const cell = document.createElement('td');
  cell.colSpan = 99;
  cell.appendChild(box);
  holderRow.appendChild(cell);
  tr.after(holderRow);
  openBox = holderRow;
  box.querySelector('textarea').focus();
  box.querySelector('.cancel').onclick = () => clearSelection();
  box.querySelector('.save').onclick = () => {
    const ta = box.querySelector('textarea');
    const body = ta.value.trim();
    if (!body) { ta.focus(); return; }
    annotations.push({ anchor: sel ? sel.current : anchor, type: 'comment', body });
    document.querySelectorAll('#diff tr.rb-selected').forEach((r) => r.classList.remove('rb-selected'));
    ta.disabled = true;
    box.querySelector('.rb-actions').innerHTML = '<span class="rb-saved">Comment added</span>';
    openBox = null;   // saved box stays in the DOM; a new click starts a fresh selection
    sel = null;
  };
}

const STATUS_LABEL = {
  submitted: 'Sent to Claude — waiting for it to start.',
  applying: 'Claude is applying your changes.',
  applied: 'Done. You can close this tab.',
};

async function finish(id, decision) {
  const hasContent = annotations.length > 0 || $('#summary').value.trim() !== '';
  if (decision === 'request-changes' && !hasContent) {
    const ta = $('#summary');
    ta.placeholder = 'Add a comment or a summary before requesting changes.';
    ta.focus();
    return;
  }
  const payload = buildReviewPayload(annotations, $('#summary').value, new Date().toISOString(), decision);
  await api(`/api/sessions/${encodeURIComponent(id)}/review`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  watchStatus(id);
}

// After submit, poll the session (browser polling costs no tokens) and reflect
// live status. A re-review re-push flips status back to `pending` with a fresh
// updatedAt — when we see that, reload into the new diff.
function watchStatus(id) {
  const paint = (status) => {
    $('#review').innerHTML = `<div class="status-panel"><p>${escapeHtml(STATUS_LABEL[status] || status)}</p></div>`;
  };
  paint('submitted');
  const h = setInterval(async () => {
    let s;
    try { s = await api(`/api/sessions/${encodeURIComponent(id)}`); } catch { return; }
    if (s.status === 'pending' && s.updatedAt !== loadedUpdatedAt) {
      clearInterval(h);
      renderReview(id);
      return;
    }
    paint(s.status);
  }, 2000);
}

function escapeHtml(s) {
  return s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

const themeBtn = document.getElementById('theme');
if (themeBtn) themeBtn.onclick = cycleTheme;
applyTheme();
window.addEventListener('popstate', route);
route();
