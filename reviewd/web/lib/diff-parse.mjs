// Parse a unified git diff into a per-file summary. Pure; no DOM.
export function parseDiffFiles(diffText) {
  const files = [];
  let cur = null;
  for (const line of diffText.split('\n')) {
    if (line.startsWith('diff --git ')) {
      const m = line.match(/ b\/(.+)$/);
      cur = { file: m ? m[1] : '', additions: 0, deletions: 0 };
      files.push(cur);
    } else if (!cur) {
      continue;
    } else if (line.startsWith('+++') || line.startsWith('---')) {
      continue;
    } else if (line.startsWith('+')) {
      cur.additions++;
    } else if (line.startsWith('-')) {
      cur.deletions++;
    }
  }
  return files;
}
