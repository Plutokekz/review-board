import { test } from 'node:test';
import assert from 'node:assert/strict';
import { buildReviewPayload } from '../web/lib/review-payload.mjs';

test('serializes annotations into the server Review shape', () => {
  const annotations = [
    {
      anchor: { file: 'a.js', side: 'new', startLine: 3, endLine: 5 },
      type: 'request_change',
      body: 'rename this',
    },
  ];
  const payload = buildReviewPayload(annotations, 'overall note', '2026-07-05T12:00:00Z', 'request-changes');
  assert.deepEqual(payload, {
    summary: 'overall note',
    decision: 'request-changes',
    submittedAt: '2026-07-05T12:00:00Z',
    comments: [
      { file: 'a.js', side: 'new', startLine: 3, endLine: 5, type: 'request_change', body: 'rename this' },
    ],
  });
});

test('missing summary becomes empty string and decision defaults to request-changes', () => {
  const p = buildReviewPayload([], undefined, 't');
  assert.equal(p.summary, '');
  assert.equal(p.decision, 'request-changes');
  assert.deepEqual(p.comments, []);
});

test('approve decision is carried through', () => {
  const p = buildReviewPayload([], 'lgtm', 't', 'approve');
  assert.equal(p.decision, 'approve');
});
