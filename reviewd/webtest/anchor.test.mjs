import { test } from 'node:test';
import assert from 'node:assert/strict';
import { anchorFromRow, mergeRange } from '../web/lib/anchor.mjs';

test('single row -> single-line anchor', () => {
  assert.deepEqual(
    anchorFromRow({ file: 'a.js', side: 'new', line: 42 }),
    { file: 'a.js', side: 'new', startLine: 42, endLine: 42 },
  );
});

test('mergeRange spans min..max regardless of click order', () => {
  const a = anchorFromRow({ file: 'a.js', side: 'new', line: 45 });
  const b = anchorFromRow({ file: 'a.js', side: 'new', line: 42 });
  assert.deepEqual(mergeRange(a, b),
    { file: 'a.js', side: 'new', startLine: 42, endLine: 45 });
});
