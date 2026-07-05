// Serialize UI annotations into the server's Review payload. Pure; no DOM.
export function buildReviewPayload(annotations, summary, submittedAt) {
  return {
    summary: summary || '',
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
