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
  const payload = buildReviewPayload(annotations, 'overall note', '2026-07-05T12:00:00Z');
  assert.deepEqual(payload, {
    summary: 'overall note',
    submittedAt: '2026-07-05T12:00:00Z',
    comments: [
      { file: 'a.js', side: 'new', startLine: 3, endLine: 5, type: 'request_change', body: 'rename this' },
    ],
  });
});

test('missing summary becomes empty string', () => {
  const p = buildReviewPayload([], undefined, 't');
  assert.equal(p.summary, '');
  assert.deepEqual(p.comments, []);
});
