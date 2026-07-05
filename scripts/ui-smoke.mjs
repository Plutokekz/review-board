// Headless-Chromium smoke test for the review-board browser UI.
//
// This covers reviewd/web/app.mjs — the DOM glue that `go test` and
// `node --test webtest/*` do NOT touch (routing, click-to-comment, range
// selection, the theme toggle). Driven over the Chrome DevTools Protocol via
// Node's built-in WebSocket (no npm deps). Exits non-zero on any failed
// assertion. Normally invoked through scripts/ui-smoke.sh, which starts an
// isolated reviewd + Chromium and sets CDP_PORT / UI_URL.
import assert from 'node:assert/strict';

const CDP = process.env.CDP_PORT || '9333';
const PAGE = process.env.UI_URL || 'http://127.0.0.1:7699/s/ui-smoke';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function getTarget() {
  for (let i = 0; i < 40; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${CDP}/json`)).json();
      const p = list.find((t) => t.type === 'page' && t.webSocketDebuggerUrl);
      if (p) return p;
    } catch { /* not up yet */ }
    await sleep(250);
  }
  throw new Error(`no CDP target on :${CDP}`);
}

const target = await getTarget();
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error('ws error')); });

let msgId = 0;
const pending = new Map();
ws.onmessage = (e) => {
  const m = JSON.parse(e.data.toString());
  if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); }
};
const cmd = (method, params = {}) =>
  new Promise((res) => { const id = ++msgId; pending.set(id, res); ws.send(JSON.stringify({ id, method, params })); });
async function evalJS(expression) {
  const m = await cmd('Runtime.evaluate', { expression, returnByValue: true });
  if (m.result.exceptionDetails) {
    const d = m.result.exceptionDetails;
    throw new Error('PAGE JS EXCEPTION: ' + ((d.exception && d.exception.description) || d.text));
  }
  return m.result.result.value;
}

await cmd('Page.enable');
await cmd('Runtime.enable');
await cmd('Page.navigate', { url: PAGE });

// wait for the diff to render
let code = 0;
for (let i = 0; i < 40; i++) {
  await sleep(250);
  try { code = await evalJS("document.querySelectorAll('#diff .d2h-code-line').length"); } catch { code = -1; }
  if (code > 0) break;
}

// 1) page loads, diff renders, theme button exists
assert.equal(await evalJS("document.body.textContent.includes('Could not load')"), false, 'page threw on load');
assert.ok(code > 0, 'no diff rendered');
assert.equal(await evalJS("!!document.getElementById('theme')"), true, 'theme button missing');

// 2) click an insertion line -> exactly one box, one highlight, zero radios
await evalJS("(()=>{const w=document.querySelector('.d2h-file-wrapper');window.__ins=[...w.querySelectorAll('tr')].filter(r=>{const n=r.querySelector('.line-num2');return r.querySelector('.d2h-code-line')&&n&&/^\\d+$/.test(n.textContent.trim());});})()");
const insN = await evalJS("window.__ins.length");
assert.ok(insN >= 3, `need >=3 insertion rows, got ${insN}`);
await evalJS("window.__ins[0].click()");
await sleep(120);
assert.equal(await evalJS("document.querySelectorAll('.comment-box').length"), 1, 'click should open one box');
assert.equal(await evalJS("document.querySelectorAll('#diff tr.rb-selected').length"), 1, 'click should highlight one line');
assert.equal(await evalJS("document.querySelectorAll('.comment-box input[type=radio]').length"), 0, 'no radios expected');

// 3) shift-click a lower line -> range highlight, still one box, box anchored to first line
await evalJS("window.__ins[2].dispatchEvent(new MouseEvent('click',{bubbles:true,shiftKey:true}))");
await sleep(120);
assert.equal(await evalJS("document.querySelectorAll('.comment-box').length"), 1, 'shift-click must not duplicate the box');
assert.ok(await evalJS("document.querySelectorAll('#diff tr.rb-selected').length") >= 3, 'range should highlight >=3 lines');
assert.equal(await evalJS("!!(window.__ins[0].nextElementSibling && window.__ins[0].nextElementSibling.querySelector('.comment-box'))"), true, 'box must stay anchored to the first-clicked line');

// 4) theme toggle: auto -> light -> dark on both the page and the diff2html wrapper
const scheme = "((document.querySelector('.d2h-wrapper')||{}).className||'').split(' ').find(c=>c.endsWith('-color-scheme'))||''";
assert.equal(await evalJS(scheme), 'd2h-auto-color-scheme', 'default should be auto');
await evalJS("document.getElementById('theme').click()");
await sleep(60);
assert.equal(await evalJS("document.documentElement.style.colorScheme"), 'light', 'first toggle -> light');
assert.equal(await evalJS(scheme), 'd2h-light-color-scheme', 'diff should be light');
await evalJS("document.getElementById('theme').click()");
await sleep(60);
assert.equal(await evalJS("document.documentElement.style.colorScheme"), 'dark', 'second toggle -> dark');
assert.equal(await evalJS(scheme), 'd2h-dark-color-scheme', 'diff should be dark');

console.log('UI SMOKE: PASS');
ws.close();
process.exit(0);
