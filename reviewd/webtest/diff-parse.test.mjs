import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseDiffFiles } from '../web/lib/diff-parse.mjs';

test('parses files and +/- counts', () => {
  const diff =
    'diff --git a/x.txt b/x.txt\n--- a/x.txt\n+++ b/x.txt\n@@ -1 +1 @@\n-old\n+new\n' +
    'diff --git a/y.txt b/y.txt\n--- a/y.txt\n+++ b/y.txt\n@@ -0,0 +1 @@\n+added\n';
  const files = parseDiffFiles(diff);
  assert.equal(files.length, 2);
  assert.deepEqual(files[0], { file: 'x.txt', additions: 1, deletions: 1 });
  assert.deepEqual(files[1], { file: 'y.txt', additions: 1, deletions: 0 });
});

test('empty diff yields no files', () => {
  assert.deepEqual(parseDiffFiles(''), []);
});
