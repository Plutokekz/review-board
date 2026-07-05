// Serialize UI annotations into the server's Review payload. Pure; no DOM.
export function buildReviewPayload(annotations, summary, submittedAt, decision) {
  return {
    summary: summary || '',
    decision: decision || 'request-changes',
    comments: annotations.map((a) => ({
      file: a.anchor.file,
      side: a.anchor.side,
      startLine: a.anchor.startLine,
      endLine: a.anchor.endLine,
      type: a.type,
      body: a.body,
    })),
    submittedAt,
  };
}
