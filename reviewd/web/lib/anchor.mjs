// Map a diff row / selection to an annotation anchor. Pure; no DOM.
export function anchorFromRow(row) {
  return { file: row.file, side: row.side, startLine: row.line, endLine: row.line };
}

export function mergeRange(a, b) {
  return {
    file: a.file,
    side: a.side,
    startLine: Math.min(a.startLine, b.startLine),
    endLine: Math.max(a.endLine, b.endLine),
  };
}
